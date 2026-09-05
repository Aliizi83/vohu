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
	if err := db.AutoMigrate(&rbac.Role{}, &rbac.Permission{}, &rbac.RolePermission{}, &rbac.UserRole{}); err != nil {
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
