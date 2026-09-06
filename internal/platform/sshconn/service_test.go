package sshconn_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/pkg/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "1P407PvOcPLeLTn+GEIwhyqWg4fV97WCZBFwlPgm1Ns="

// rbac.ResourcePermission is migrated here too (not just SSHConnection) —
// ListForCaller/GetByIDForCaller's non-bypass path runs a real SQL EXISTS
// subquery against that table, so the filtering tests below need it to
// actually exist. Importing rbac from a _test.go file doesn't reintroduce
// the production coupling sshconn's own code deliberately avoids — this
// is test setup, not the module depending on rbac's package at runtime.
func setupSSHConnTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&sshconn.SSHConnection{}, &rbac.ResourcePermission{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

// grantCall records the arguments of the last GrantCreatorAccess
// invocation so tests can assert the auto-grant behavior without needing
// a real rbac.Service.
type grantCall struct {
	userID       uint
	resourceType string
	resourceID   uint
	effect       string
}

func alwaysHasPermission(ctx context.Context, userID uint, key string) (bool, error) {
	return true, nil
}

func neverHasPermission(ctx context.Context, userID uint, key string) (bool, error) {
	return false, nil
}

func newTestService(t *testing.T) (sshconn.Service, *[]grantCall) {
	t.Helper()
	service, _, calls := newTestServiceWithChecks(t, alwaysHasPermission, nil)
	return service, calls
}

func newTestServiceWithChecks(
	t *testing.T,
	hasPermission shared.PermissionCheck,
	hasAccessLevel shared.AccessLevelCheck,
) (sshconn.Service, *gorm.DB, *[]grantCall) {
	t.Helper()

	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	calls := &[]grantCall{}
	grant := func(ctx context.Context, userID uint, resourceType string, resourceID uint, effect string) error {
		*calls = append(*calls, grantCall{userID, resourceType, resourceID, effect})
		return nil
	}

	db := setupSSHConnTestDB(t)
	repo := sshconn.NewRepository(db)
	return sshconn.NewService(repo, box, grant, hasPermission, hasAccessLevel), db, calls
}

func TestCreate_EncryptsSecretAndGrantsCreatorAccess(t *testing.T) {
	service, calls := newTestService(t)
	ctx := context.Background()

	conn, err := service.Create(ctx, 7, sshconn.CreateSSHConnectionRequest{
		Name:       "prod-box",
		Host:       "10.0.0.5",
		Username:   "deploy",
		AuthMethod: sshconn.AuthPassword,
		Secret:     "hunter2",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if conn.EncryptedSecret == "hunter2" {
		t.Fatal("expected the stored secret to be encrypted, not plaintext")
	}
	if conn.Port != 22 {
		t.Fatalf("expected default port 22, got %d", conn.Port)
	}
	if conn.CreatedByUserID != 7 {
		t.Fatalf("expected CreatedByUserID 7, got %d", conn.CreatedByUserID)
	}

	if len(*calls) != 1 {
		t.Fatalf("expected exactly one grant-access call, got %d", len(*calls))
	}
	got := (*calls)[0]
	if got.userID != 7 || got.resourceType != sshconn.ResourceTypeSSHConnection || got.resourceID != conn.ID || got.effect != "manage" {
		t.Fatalf("unexpected grant call: %+v", got)
	}

	plaintext, err := service.DecryptSecret(conn)
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if plaintext != "hunter2" {
		t.Fatalf("expected decrypted secret %q, got %q", "hunter2", plaintext)
	}
}

func TestCreate_RespectsExplicitPort(t *testing.T) {
	service, _ := newTestService(t)

	conn, err := service.Create(context.Background(), 1, sshconn.CreateSSHConnectionRequest{
		Name:       "custom-port",
		Host:       "example.com",
		Port:       2222,
		Username:   "root",
		AuthMethod: sshconn.AuthPrivateKey,
		Secret:     "-----BEGIN KEY-----",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if conn.Port != 2222 {
		t.Fatalf("expected port 2222, got %d", conn.Port)
	}
}

func TestUpdate_WithoutSecretKeepsExistingEncryptedValue(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name:       "box",
		Host:       "1.2.3.4",
		Username:   "user",
		AuthMethod: sshconn.AuthPassword,
		Secret:     "original-secret",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, conn.ID, sshconn.UpdateSSHConnectionRequest{
		Name: "renamed-box",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Name != "renamed-box" {
		t.Fatalf("expected name to update, got %q", updated.Name)
	}

	plaintext, err := service.DecryptSecret(updated)
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if plaintext != "original-secret" {
		t.Fatalf("expected the secret to be left untouched, got %q", plaintext)
	}
}

func TestUpdate_WithNewSecretReEncrypts(t *testing.T) {
	service, _ := newTestService(t)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name:       "box",
		Host:       "1.2.3.4",
		Username:   "user",
		AuthMethod: sshconn.AuthPassword,
		Secret:     "old-secret",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, conn.ID, sshconn.UpdateSSHConnectionRequest{
		Secret: "new-secret",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	plaintext, err := service.DecryptSecret(updated)
	if err != nil {
		t.Fatalf("DecryptSecret failed: %v", err)
	}
	if plaintext != "new-secret" {
		t.Fatalf("expected the secret to be updated, got %q", plaintext)
	}
}

func TestDelete_NotFound(t *testing.T) {
	service, _ := newTestService(t)

	err := service.Delete(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestListForCaller_WithFlatPermission_SeesEverything(t *testing.T) {
	service, _, _ := newTestServiceWithChecks(t, alwaysHasPermission, nil)
	ctx := context.Background()

	if _, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := service.Create(ctx, 2, sshconn.CreateSSHConnectionRequest{
		Name: "b", Host: "2.2.2.2", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Caller 999 owns neither connection, but holds the flat permission
	// (alwaysHasPermission) — should still see both, unfiltered.
	items, total, err := service.ListForCaller(ctx, 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected a flat-permission holder to see both connections, got total=%d len=%d", total, len(items))
	}
}

func TestListForCaller_WithoutFlatPermission_SeesOnlyAccessible(t *testing.T) {
	service, db, _ := newTestServiceWithChecks(t, neverHasPermission, nil)
	ctx := context.Background()

	connA, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	connB, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "b", Host: "2.2.2.2", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// The stub GrantCreatorAccess never actually writes a row, so seed the
	// real resource_permissions table directly: caller 42 can read connA
	// only, not connB.
	if err := db.Create(&rbac.ResourcePermission{
		UserID: 42, ResourceType: sshconn.ResourceTypeSSHConnection, ResourceID: connA.ID, Level: rbac.AccessRead,
	}).Error; err != nil {
		t.Fatalf("seeding resource_permissions failed: %v", err)
	}
	if err := db.Create(&rbac.ResourcePermission{
		UserID: 42, ResourceType: sshconn.ResourceTypeSSHConnection, ResourceID: connB.ID, Level: rbac.AccessForbidden,
	}).Error; err != nil {
		t.Fatalf("seeding resource_permissions failed: %v", err)
	}

	items, total, err := service.ListForCaller(ctx, 42, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected exactly 1 accessible connection, got total=%d len=%d", total, len(items))
	}
	if items[0].ID != connA.ID {
		t.Fatalf("expected connection %d (Read-granted), got %d", connA.ID, items[0].ID)
	}
}

func TestListForCaller_WithoutFlatPermissionOrAnyGrant_SeesNothing(t *testing.T) {
	service, _, _ := newTestServiceWithChecks(t, neverHasPermission, nil)
	ctx := context.Background()

	if _, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	items, total, err := service.ListForCaller(ctx, 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected zero connections for a caller with no flat permission and no grants, got total=%d len=%d", total, len(items))
	}
}

func TestGetByIDForCaller_WithFlatPermission_SeesAnyConnection(t *testing.T) {
	service, _, _ := newTestServiceWithChecks(t, alwaysHasPermission, nil)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := service.GetByIDForCaller(ctx, 999, conn.ID)
	if err != nil {
		t.Fatalf("GetByIDForCaller failed: %v", err)
	}
	if got.ID != conn.ID {
		t.Fatalf("expected connection %d, got %d", conn.ID, got.ID)
	}
}

func TestGetByIDForCaller_WithoutFlatPermissionOrGrant_NotFound(t *testing.T) {
	denyLevel := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return false, nil
	}
	service, _, _ := newTestServiceWithChecks(t, neverHasPermission, denyLevel)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = service.GetByIDForCaller(ctx, 999, conn.ID)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound (existence not leaked), got %v", err)
	}
}

func TestGetByIDForCaller_WithoutFlatPermissionButWithGrant_Succeeds(t *testing.T) {
	allowLevel := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return true, nil
	}
	service, _, _ := newTestServiceWithChecks(t, neverHasPermission, allowLevel)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := service.GetByIDForCaller(ctx, 42, conn.ID)
	if err != nil {
		t.Fatalf("GetByIDForCaller failed: %v", err)
	}
	if got.ID != conn.ID {
		t.Fatalf("expected connection %d, got %d", conn.ID, got.ID)
	}
}
