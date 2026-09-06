package rbac_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRBACTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&rbac.Role{}, &rbac.Permission{}, &rbac.RolePermission{}, &rbac.UserRole{}, &rbac.ResourcePermission{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// seedUserWithPermission creates a role holding the given permission and
// assigns it to userID, exercising the full role -> permission ->
// role_permission -> user_role chain HasPermission has to join across.
func seedUserWithPermission(t *testing.T, service rbac.Service, userID uint, permissionKey string) {
	t.Helper()
	ctx := context.Background()

	role, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "test-role"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	permission, err := service.CreatePermission(ctx, rbac.CreatePermissionRequest{Key: permissionKey})
	if err != nil {
		t.Fatalf("CreatePermission failed: %v", err)
	}

	if err := service.GrantPermissionToRole(ctx, role.ID, permission.ID); err != nil {
		t.Fatalf("GrantPermissionToRole failed: %v", err)
	}

	if err := service.AssignRoleToUser(ctx, userID, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}
}

func TestHasPermission_GrantedThroughRole(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	seedUserWithPermission(t, service, 1, "widget:create")

	allowed, err := service.HasPermission(context.Background(), 1, "widget:create")
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected the user to have widget:create through their role")
	}
}

func TestHasPermission_DeniedForDifferentKey(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	seedUserWithPermission(t, service, 1, "widget:create")

	allowed, err := service.HasPermission(context.Background(), 1, "widget:delete")
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if allowed {
		t.Fatal("expected the user to NOT have a permission nobody granted them")
	}
}

func TestHasPermission_DeniedForUnknownUser(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	seedUserWithPermission(t, service, 1, "widget:create")

	allowed, err := service.HasPermission(context.Background(), 999, "widget:create")
	if err != nil {
		t.Fatalf("HasPermission failed: %v", err)
	}
	if allowed {
		t.Fatal("expected a user with no role assignments to have no permissions")
	}
}

func TestRequirePolicy_ForwardsToHasPermission(t *testing.T) {
	// shared.RequirePolicy is exercised end-to-end via the handler
	// integration test; this checks the Policy method it wraps returns
	// the right answer directly against a seeded DB.
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	seedUserWithPermission(t, service, 1, "rbac:manage")
	policy := rbac.NewPolicy(service)

	allowed, err := policy.CanManage(context.Background(), 1)
	if err != nil {
		t.Fatalf("CanManage failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected CanManage to be true for a user granted rbac:manage")
	}

	allowed, err = policy.CanManage(context.Background(), 2)
	if err != nil {
		t.Fatalf("CanManage failed: %v", err)
	}
	if allowed {
		t.Fatal("expected CanManage to be false for a user with no permissions")
	}
}

func TestCreateRole_DuplicateNameFails(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if _, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "dup"}); err != nil {
		t.Fatalf("first CreateRole failed: %v", err)
	}

	_, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "dup"})
	if err != rbac.ErrRoleExists {
		t.Fatalf("expected ErrRoleExists, got %v", err)
	}
}

func TestDeleteRole_NotFound(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))

	err := service.DeleteRole(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestHasAccessLevel_DefaultDenyWithNoRow(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))

	allowed, err := service.HasAccessLevel(context.Background(), 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected default deny when no resource_permission row exists")
	}
}

func TestHasAccessLevel_HigherGrantSatisfiesLowerRequirement(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessManage); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	for _, required := range []rbac.AccessLevel{rbac.AccessRead, rbac.AccessWrite, rbac.AccessManage} {
		allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, required)
		if err != nil {
			t.Fatalf("HasAccessLevel failed: %v", err)
		}
		if !allowed {
			t.Fatalf("expected a Manage grant to satisfy a %q requirement", required)
		}
	}

	// A different resource ID (same type) must stay denied.
	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 6, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected a grant on resource 5 to not leak into resource 6")
	}
}

func TestHasAccessLevel_LowerGrantDoesNotSatisfyHigherRequirement(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessRead); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessWrite)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected a Read grant to NOT satisfy a Write requirement")
	}
}

func TestHasAccessLevel_Forbidden(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessForbidden); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected AccessForbidden to deny even the lowest requirement")
	}
}

func TestGrantResourceAccess_UpsertsOnRepeatGrant(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessManage); err != nil {
		t.Fatalf("first grant failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessForbidden); err != nil {
		t.Fatalf("second grant failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected the second grant (Forbidden) to overwrite the first (Manage), not add a second row")
	}

	list, total, err := service.ListResourcePermissions(ctx, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListResourcePermissions failed: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Fatalf("expected exactly 1 row after two grants to the same (user, resource), got total=%d len=%d", total, len(list))
	}
}

func TestRevokeResourceAccess_FallsBackToDefaultDeny(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessWrite); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	list, _, err := service.ListResourcePermissions(ctx, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListResourcePermissions failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 row, got %d", len(list))
	}

	if err := service.RevokeResourceAccess(ctx, list[0].ID); err != nil {
		t.Fatalf("RevokeResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected access to fall back to default-deny after revoking the only grant")
	}
}

func TestRevokeResourceAccess_NotFound(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))

	err := service.RevokeResourceAccess(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestAccessLevel_Satisfies(t *testing.T) {
	cases := []struct {
		level    rbac.AccessLevel
		required rbac.AccessLevel
		want     bool
	}{
		{rbac.AccessManage, rbac.AccessRead, true},
		{rbac.AccessManage, rbac.AccessWrite, true},
		{rbac.AccessManage, rbac.AccessManage, true},
		{rbac.AccessWrite, rbac.AccessRead, true},
		{rbac.AccessWrite, rbac.AccessWrite, true},
		{rbac.AccessWrite, rbac.AccessManage, false},
		{rbac.AccessRead, rbac.AccessRead, true},
		{rbac.AccessRead, rbac.AccessWrite, false},
		{rbac.AccessForbidden, rbac.AccessRead, false},
		{rbac.AccessForbidden, rbac.AccessForbidden, false},
	}

	for _, tc := range cases {
		got := tc.level.Satisfies(tc.required)
		if got != tc.want {
			t.Errorf("%q.Satisfies(%q) = %v, want %v", tc.level, tc.required, got, tc.want)
		}
	}
}

func TestListPermissionKeysForUser_ReturnsEveryKeyThroughAnyRole(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	// seedUserWithPermission always creates a role named "test-role", so
	// granting a second key goes through it directly instead of calling
	// that helper twice (which would collide on the role name).
	role, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "test-role"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	for _, key := range []string{"widget:create", "widget:delete"} {
		permission, err := service.CreatePermission(ctx, rbac.CreatePermissionRequest{Key: key})
		if err != nil {
			t.Fatalf("CreatePermission failed: %v", err)
		}
		if err := service.GrantPermissionToRole(ctx, role.ID, permission.ID); err != nil {
			t.Fatalf("GrantPermissionToRole failed: %v", err)
		}
	}
	if err := service.AssignRoleToUser(ctx, 1, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	keys, err := service.ListPermissionKeysForUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListPermissionKeysForUser failed: %v", err)
	}

	want := map[string]bool{"widget:create": true, "widget:delete": true}
	if len(keys) != len(want) {
		t.Fatalf("expected %d keys, got %d: %v", len(want), len(keys), keys)
	}
	for _, k := range keys {
		if !want[k] {
			t.Fatalf("unexpected key %q in %v", k, keys)
		}
	}
}

func TestListPermissionKeysForUser_EmptyForUserWithNoRoles(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))

	keys, err := service.ListPermissionKeysForUser(context.Background(), 999)
	if err != nil {
		t.Fatalf("ListPermissionKeysForUser failed: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected no keys for a user with no roles, got %v", keys)
	}
}

func TestListResourceAccessForUser_ReturnsOnlyThatUsersGrants(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 5, rbac.AccessWrite); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, 1, "ssh_connection", 6, rbac.AccessRead); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, 2, "ssh_connection", 5, rbac.AccessManage); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	grants, err := service.ListResourceAccessForUser(ctx, 1)
	if err != nil {
		t.Fatalf("ListResourceAccessForUser failed: %v", err)
	}
	if len(grants) != 2 {
		t.Fatalf("expected exactly 2 grants for user 1, got %d: %+v", len(grants), grants)
	}
	for _, g := range grants {
		if g.UserID != 1 {
			t.Fatalf("expected only user 1's grants, got one for user %d", g.UserID)
		}
	}
}

func TestListResourceAccessForUser_EmptyForUserWithNoGrants(t *testing.T) {
	service := rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))

	grants, err := service.ListResourceAccessForUser(context.Background(), 999)
	if err != nil {
		t.Fatalf("ListResourceAccessForUser failed: %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("expected no grants, got %+v", grants)
	}
}
