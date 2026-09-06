package sshconn_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/pkg/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "1P407PvOcPLeLTn+GEIwhyqWg4fV97WCZBFwlPgm1Ns="

func setupSSHConnTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&sshconn.SSHConnection{}); err != nil {
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
	level        string
	effect       string
}

func newTestService(t *testing.T, hasAccessLevel shared.AccessLevelCheck) (sshconn.Service, *[]grantCall) {
	t.Helper()

	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	calls := &[]grantCall{}
	grant := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string, effect string) error {
		*calls = append(*calls, grantCall{userID, resourceType, resourceID, level, effect})
		return nil
	}

	repo := sshconn.NewRepository(setupSSHConnTestDB(t))
	return sshconn.NewService(repo, box, grant, hasAccessLevel), calls
}

func denyAllLevels(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func allowAllLevels(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

func TestCreate_EncryptsSecretAndGrantsCreatorManageAccess(t *testing.T) {
	service, calls := newTestService(t, denyAllLevels)
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
	if got.userID != 7 || got.resourceType != sshconn.ResourceTypeSSHConnection || got.resourceID != conn.ID ||
		got.level != "manage" || got.effect != "accepted" {
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
	service, _ := newTestService(t, denyAllLevels)

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
	service, _ := newTestService(t, denyAllLevels)
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
	service, _ := newTestService(t, denyAllLevels)
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
	service, _ := newTestService(t, denyAllLevels)

	err := service.Delete(context.Background(), 9999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestListForCaller_WithWildcardReadAccess_SeesEverything(t *testing.T) {
	service, _ := newTestService(t, allowAllLevels)
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

	// Caller 999 owns neither connection, but the stub grants access to
	// everything (simulating a wildcard "read" grant) — should see both.
	items, total, err := service.ListForCaller(ctx, 999, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("expected a wildcard-read holder to see both connections, got total=%d len=%d", total, len(items))
	}
}

func TestListForCaller_WithoutWildcardAccess_SeesOnlyGrantedRows(t *testing.T) {
	ctx := context.Background()

	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}
	repo := sshconn.NewRepository(setupSSHConnTestDB(t))
	noopGrant := func(context.Context, uint, string, uint, string, string) error { return nil }

	// Two service values sharing the same underlying repo/DB, differing
	// only in hasAccessLevel — creation uses a permissive one (in
	// production, Create is gated by the wildcard "write" route
	// middleware, not exercised here); listing uses a restrictive one
	// that only allows the specific connection this test expects visible.
	creator := sshconn.NewService(repo, box, noopGrant, allowAllLevels)

	connA, err := creator.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := creator.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "b", Host: "2.2.2.2", Username: "u", AuthMethod: sshconn.AuthPassword, Secret: "s",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	restrictive := sshconn.NewService(repo, box, noopGrant, func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		if resourceID == shared.WildcardResourceID {
			return false, nil
		}
		return resourceID == connA.ID, nil
	})

	items, total, err := restrictive.ListForCaller(ctx, 42, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListForCaller failed: %v", err)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected exactly 1 accessible connection, got total=%d len=%d", total, len(items))
	}
	if items[0].ID != connA.ID {
		t.Fatalf("expected connection %d (granted), got %d", connA.ID, items[0].ID)
	}
}

func TestListForCaller_NoAccessAtAll_SeesNothing(t *testing.T) {
	service, _ := newTestService(t, denyAllLevels)
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
		t.Fatalf("expected zero connections for a caller with no access at all, got total=%d len=%d", total, len(items))
	}
}
