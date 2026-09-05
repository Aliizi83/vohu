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
	effect       string
}

func newTestService(t *testing.T) (sshconn.Service, *[]grantCall) {
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

	repo := sshconn.NewRepository(setupSSHConnTestDB(t))
	return sshconn.NewService(repo, box, grant), calls
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
	if got.userID != 7 || got.resourceType != sshconn.ResourceTypeSSHConnection || got.resourceID != conn.ID || got.effect != "accepted" {
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
