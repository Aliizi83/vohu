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
	if err := db.AutoMigrate(&rbac.Role{}, &rbac.UserRole{}, &rbac.ResourceAccess{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) rbac.Service {
	t.Helper()
	return rbac.NewService(rbac.NewRepository(setupRBACTestDB(t)))
}

func TestCreateRole_DuplicateNameFails(t *testing.T) {
	service := newTestService(t)
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
	service := newTestService(t)

	err := service.DeleteRole(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestHasAccessLevel_DefaultDenyWithNoRowAnywhere(t *testing.T) {
	service := newTestService(t)

	allowed, err := service.HasAccessLevel(context.Background(), 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected default deny when no row exists anywhere")
	}
}

func TestHasAccessLevel_DirectUserGrant(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	for _, tc := range []struct {
		required rbac.AccessLevel
		want     bool
	}{
		{rbac.AccessRead, true},
		{rbac.AccessWrite, true},
		{rbac.AccessManage, false},
	} {
		allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, tc.required)
		if err != nil {
			t.Fatalf("HasAccessLevel failed: %v", err)
		}
		if allowed != tc.want {
			t.Fatalf("required=%q: got %v, want %v", tc.required, allowed, tc.want)
		}
	}

	// A different resource ID must stay denied — the grant doesn't leak.
	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 6, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected a grant on resource 5 to not leak into resource 6")
	}
}

func TestHasAccessLevel_DirectUserProhibited(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessManage, rbac.EffectProhibited); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected an explicit prohibited row to deny even the lowest requirement")
	}
}

func TestHasAccessLevel_WildcardUserGrantSatisfiesAnyRealID(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", shared.WildcardResourceID, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	for _, resourceID := range []uint{5, 6, 999} {
		allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", resourceID, rbac.AccessManage)
		if err != nil {
			t.Fatalf("HasAccessLevel failed: %v", err)
		}
		if !allowed {
			t.Fatalf("expected the wildcard grant to satisfy resource id %d", resourceID)
		}
	}
}

func TestHasAccessLevel_ExactUserRowOverridesWildcard(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", shared.WildcardResourceID, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("wildcard grant failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessRead, rbac.EffectProhibited); err != nil {
		t.Fatalf("exact prohibited grant failed: %v", err)
	}

	// The exact row on resource 5 (prohibited) beats the broader wildcard grant.
	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected the exact prohibited row to override the wildcard accepted grant")
	}

	// A different resource ID still falls back to the wildcard grant.
	allowed, err = service.HasAccessLevel(ctx, 1, "ssh_connection", 6, rbac.AccessManage)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected resource 6 (no exact row) to still fall back to the wildcard grant")
	}
}

func TestHasAccessLevel_ViaRoleMembership(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	role, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "support"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 1, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeRole, role.ID, "ssh_connection", 5, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessWrite)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected access granted to the user's role to apply to the user")
	}

	// A user with no roles at all gets nothing from this grant.
	allowed, err = service.HasAccessLevel(ctx, 2, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected a user with no roles to have no access via someone else's role grant")
	}
}

func TestHasAccessLevel_MultiRoleProhibitedVetoesSufficientGrantFromAnotherRole(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	roleA, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "role-a"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	roleB, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "role-b"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 1, roleA.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 1, roleB.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	// role A grants write, role B explicitly prohibits — prohibited wins.
	if err := service.GrantResourceAccess(ctx, rbac.GranteeRole, roleA.ID, "ssh_connection", 5, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess (A) failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeRole, roleB.ID, "ssh_connection", 5, rbac.AccessManage, rbac.EffectProhibited); err != nil {
		t.Fatalf("GrantResourceAccess (B) failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected role B's prohibited row to veto role A's sufficient grant")
	}
}

func TestHasAccessLevel_RoleCascadeToMembers_WithPerUserException(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	// caller (userID 100) is granted read access to every user holding
	// the "regular" role.
	regular, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "regular"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 10, regular.ID); err != nil { // target user A
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 11, regular.ID); err != nil { // target user B
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 100, "role", regular.ID, rbac.AccessRead, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess (role cascade) failed: %v", err)
	}

	// Caller can reach both members of the role via the cascade.
	for _, targetUserID := range []uint{10, 11} {
		allowed, err := service.HasAccessLevel(ctx, 100, "user", targetUserID, rbac.AccessRead)
		if err != nil {
			t.Fatalf("HasAccessLevel failed: %v", err)
		}
		if !allowed {
			t.Fatalf("expected the role cascade to grant access to user %d", targetUserID)
		}
	}

	// Now carve user 11 back out with an explicit, more specific prohibited row.
	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 100, "user", 11, rbac.AccessRead, rbac.EffectProhibited); err != nil {
		t.Fatalf("GrantResourceAccess (exception) failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 100, "user", 11, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected the explicit per-user prohibited row to carve user 11 out of the role cascade")
	}

	// User 10 is unaffected — still reachable via the cascade.
	allowed, err = service.HasAccessLevel(ctx, 100, "user", 10, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if !allowed {
		t.Fatal("expected user 10 to remain reachable via the role cascade")
	}
}

func TestHasAccessLevel_CascadeOnlyAppliesToUserResourceType(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	role, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "role-x"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}

	// Caller has read access to the role itself, but that must not leak
	// into unrelated resource types just because a role ID happens to
	// match a real resource ID of a different type.
	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "role", role.ID, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", role.ID, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected access to a role resource to not cascade into an unrelated ssh_connection resource")
	}
}

func TestGrantResourceAccess_UpsertsOnRepeatGrant(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("first grant failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessRead, rbac.EffectProhibited); err != nil {
		t.Fatalf("second grant failed: %v", err)
	}

	allowed, err := service.HasAccessLevel(ctx, 1, "ssh_connection", 5, rbac.AccessRead)
	if err != nil {
		t.Fatalf("HasAccessLevel failed: %v", err)
	}
	if allowed {
		t.Fatal("expected the second grant (prohibited) to overwrite the first (accepted), not add a second row")
	}

	list, err := service.MyResourceAccess(ctx, 1)
	if err != nil {
		t.Fatalf("MyResourceAccess failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected exactly 1 row after two grants to the same tuple, got %d", len(list))
	}
}

func TestRevokeResourceAccess_FallsBackToDefaultDeny(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	list, err := service.MyResourceAccess(ctx, 1)
	if err != nil {
		t.Fatalf("MyResourceAccess failed: %v", err)
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
	service := newTestService(t)

	err := service.RevokeResourceAccess(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestMyResourceAccess_IncludesDirectAndRoleGrants(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	role, err := service.CreateRole(ctx, rbac.CreateRoleRequest{Name: "some-role"})
	if err != nil {
		t.Fatalf("CreateRole failed: %v", err)
	}
	if err := service.AssignRoleToUser(ctx, 1, role.ID); err != nil {
		t.Fatalf("AssignRoleToUser failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 5, rbac.AccessRead, rbac.EffectAccepted); err != nil {
		t.Fatalf("direct grant failed: %v", err)
	}
	if err := service.GrantResourceAccess(ctx, rbac.GranteeRole, role.ID, "ssh_connection", 6, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("role grant failed: %v", err)
	}

	list, err := service.MyResourceAccess(ctx, 1)
	if err != nil {
		t.Fatalf("MyResourceAccess failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 rows (1 direct + 1 via role), got %d: %+v", len(list), list)
	}
}

func TestMyLevel_ReturnsBestLevelOrNotOK(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	_, ok, err := service.MyLevel(ctx, 1, "ssh_connection")
	if err != nil {
		t.Fatalf("MyLevel failed: %v", err)
	}
	if ok {
		t.Fatal("expected ok=false when no wildcard grant exists")
	}

	if err := service.GrantResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", shared.WildcardResourceID, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("GrantResourceAccess failed: %v", err)
	}

	level, ok, err := service.MyLevel(ctx, 1, "ssh_connection")
	if err != nil {
		t.Fatalf("MyLevel failed: %v", err)
	}
	if !ok || level != rbac.AccessWrite {
		t.Fatalf("expected (write, true), got (%q, %v)", level, ok)
	}
}
