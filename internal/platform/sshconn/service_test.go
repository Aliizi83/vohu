package sshconn_test

import (
	"context"
	"errors"
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

// testConnCall records the arguments of the last TestConnectionFunc
// invocation so tests can assert Create passes through the right values
// without needing a real SSH server.
type testConnCall struct {
	host       string
	port       int
	username   string
	privateKey string
}

func alwaysSucceeds(ctx context.Context, host string, port int, username string, privateKey string) error {
	return nil
}

var errConnectionUnreachable = errors.New("connection refused")

func alwaysFails(ctx context.Context, host string, port int, username string, privateKey string) error {
	return errConnectionUnreachable
}

func newTestService(t *testing.T, hasAccessLevel shared.AccessLevelCheck) (sshconn.Service, *[]grantCall) {
	t.Helper()
	service, calls, _ := newTestServiceWithTestConnection(t, hasAccessLevel, alwaysSucceeds)
	return service, calls
}

func newTestServiceWithTestConnection(
	t *testing.T, hasAccessLevel shared.AccessLevelCheck, testConnection sshconn.TestConnectionFunc,
) (sshconn.Service, *[]grantCall, *[]testConnCall) {
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

	testCalls := &[]testConnCall{}
	spyTestConnection := func(ctx context.Context, host string, port int, username string, privateKey string) error {
		*testCalls = append(*testCalls, testConnCall{host, port, username, privateKey})
		return testConnection(ctx, host, port, username, privateKey)
	}

	repo := sshconn.NewRepository(setupSSHConnTestDB(t))
	return sshconn.NewService(repo, box, grant, hasAccessLevel, spyTestConnection), calls, testCalls
}

func denyAllLevels(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func allowAllLevels(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

func TestCreate_EncryptsPrivateKeyAndGrantsCreatorManageAccess(t *testing.T) {
	service, calls := newTestService(t, denyAllLevels)
	ctx := context.Background()

	conn, err := service.Create(ctx, 7, sshconn.CreateSSHConnectionRequest{
		Name:       "prod-box",
		Host:       "10.0.0.5",
		Username:   "deploy",
		PrivateKey: "-----BEGIN KEY-----",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if conn.EncryptedPrivateKey == "-----BEGIN KEY-----" {
		t.Fatal("expected the stored private key to be encrypted, not plaintext")
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

	plaintext, err := service.DecryptPrivateKey(conn)
	if err != nil {
		t.Fatalf("DecryptPrivateKey failed: %v", err)
	}
	if plaintext != "-----BEGIN KEY-----" {
		t.Fatalf("expected decrypted private key %q, got %q", "-----BEGIN KEY-----", plaintext)
	}
}

func TestCreate_RespectsExplicitPort(t *testing.T) {
	service, _ := newTestService(t, denyAllLevels)

	conn, err := service.Create(context.Background(), 1, sshconn.CreateSSHConnectionRequest{
		Name:       "custom-port",
		Host:       "example.com",
		Port:       2222,
		Username:   "root",
		PrivateKey: "-----BEGIN KEY-----",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if conn.Port != 2222 {
		t.Fatalf("expected port 2222, got %d", conn.Port)
	}
}

// TestCreate_TestsConnectionWithTheRightParamsBeforeSaving is the actual
// feature this change exists for: Create must verify the connection
// really works — dialing with the given host/port/username/private key —
// before ever writing a row.
func TestCreate_TestsConnectionWithTheRightParamsBeforeSaving(t *testing.T) {
	service, _, testCalls := newTestServiceWithTestConnection(t, allowAllLevels, alwaysSucceeds)
	ctx := context.Background()

	if _, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name:       "box",
		Host:       "10.0.0.9",
		Port:       2200,
		Username:   "deploy",
		PrivateKey: "-----BEGIN KEY-----",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if len(*testCalls) != 1 {
		t.Fatalf("expected exactly one connection test, got %d", len(*testCalls))
	}
	got := (*testCalls)[0]
	if got.host != "10.0.0.9" || got.port != 2200 || got.username != "deploy" || got.privateKey != "-----BEGIN KEY-----" {
		t.Fatalf("unexpected connection test call: %+v", got)
	}
}

// TestCreate_TestsConnectionWithDefaultPortWhenOmitted confirms the test
// call sees the *resolved* port (22, not 0) when the caller didn't
// specify one — Create's default-port logic must run before the test,
// not after.
func TestCreate_TestsConnectionWithDefaultPortWhenOmitted(t *testing.T) {
	service, _, testCalls := newTestServiceWithTestConnection(t, allowAllLevels, alwaysSucceeds)
	ctx := context.Background()

	if _, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "box", Host: "10.0.0.9", Username: "deploy", PrivateKey: "-----BEGIN KEY-----",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if len(*testCalls) != 1 || (*testCalls)[0].port != 22 {
		t.Fatalf("expected the connection test to see the default port 22, got %+v", *testCalls)
	}
}

// TestCreate_FailedConnectionTestIsNotPersisted is the other half of the
// feature: a connection that doesn't actually work must never reach the
// database, and the creator must never be granted access to a
// nonexistent row.
func TestCreate_FailedConnectionTestIsNotPersisted(t *testing.T) {
	service, calls, _ := newTestServiceWithTestConnection(t, allowAllLevels, alwaysFails)
	ctx := context.Background()

	_, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "box", Host: "10.0.0.9", Username: "deploy", PrivateKey: "-----BEGIN KEY-----",
	})
	if !errors.Is(err, sshconn.ErrConnectionTestFailed) {
		t.Fatalf("expected ErrConnectionTestFailed, got %v", err)
	}
	if !errors.Is(err, errConnectionUnreachable) {
		t.Fatalf("expected the underlying dial error to be wrapped, got %v", err)
	}

	if len(*calls) != 0 {
		t.Fatalf("expected no grant-access call for a connection that was never created, got %d", len(*calls))
	}

	items, total, err := service.List(ctx, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected nothing to be persisted after a failed connection test, got total=%d len=%d", total, len(items))
	}
}

func TestUpdate_WithoutPrivateKeyKeepsExistingEncryptedValue(t *testing.T) {
	service, _ := newTestService(t, denyAllLevels)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "box", Host: "1.2.3.4", Username: "user", PrivateKey: "original-key",
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

	plaintext, err := service.DecryptPrivateKey(updated)
	if err != nil {
		t.Fatalf("DecryptPrivateKey failed: %v", err)
	}
	if plaintext != "original-key" {
		t.Fatalf("expected the private key to be left untouched, got %q", plaintext)
	}
}

func TestUpdate_WithNewPrivateKeyReEncrypts(t *testing.T) {
	service, _ := newTestService(t, denyAllLevels)
	ctx := context.Background()

	conn, err := service.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "box", Host: "1.2.3.4", Username: "user", PrivateKey: "old-key",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	updated, err := service.Update(ctx, conn.ID, sshconn.UpdateSSHConnectionRequest{
		PrivateKey: "new-key",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	plaintext, err := service.DecryptPrivateKey(updated)
	if err != nil {
		t.Fatalf("DecryptPrivateKey failed: %v", err)
	}
	if plaintext != "new-key" {
		t.Fatalf("expected the private key to be updated, got %q", plaintext)
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
		Name: "a", Host: "1.1.1.1", Username: "u", PrivateKey: "k",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := service.Create(ctx, 2, sshconn.CreateSSHConnectionRequest{
		Name: "b", Host: "2.2.2.2", Username: "u", PrivateKey: "k",
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
	creator := sshconn.NewService(repo, box, noopGrant, allowAllLevels, alwaysSucceeds)

	connA, err := creator.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "a", Host: "1.1.1.1", Username: "u", PrivateKey: "k",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := creator.Create(ctx, 1, sshconn.CreateSSHConnectionRequest{
		Name: "b", Host: "2.2.2.2", Username: "u", PrivateKey: "k",
	}); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	restrictive := sshconn.NewService(repo, box, noopGrant, func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		if resourceID == shared.WildcardResourceID {
			return false, nil
		}
		return resourceID == connA.ID, nil
	}, alwaysSucceeds)

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
		Name: "a", Host: "1.1.1.1", Username: "u", PrivateKey: "k",
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
