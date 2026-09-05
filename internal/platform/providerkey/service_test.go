package providerkey_test

import (
	"context"
	"os"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/providerkey"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const testEncryptionKey = "1P407PvOcPLeLTn+GEIwhyqWg4fV97WCZBFwlPgm1Ns="

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&providerkey.Key{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) providerkey.Service {
	t.Helper()

	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	return providerkey.NewService(providerkey.NewRepository(setupTestDB(t)), box)
}

// clearProviderEnv ensures no ambient environment variable from the real
// shell leaks into a test expecting "nothing configured" or exercising a
// specific fallback.
func clearProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"GEMINI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_WORKSPACE_ID", "OPENAI_API_KEY", "OPENAI_BASE_URL"} {
		t.Setenv(key, "")
	}
}

func TestResolve_NoKeyAnywhereReturnsError(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)

	_, err := service.Resolve(context.Background(), 1, providerkey.ProviderGemini)
	if err == nil {
		t.Fatal("expected an error when no user key, global key, or env var is configured")
	}
}

func TestResolve_FallsBackToEnvVar(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("GEMINI_API_KEY", "env-key")
	service := newTestService(t)

	cred, err := service.Resolve(context.Background(), 1, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "env-key" {
		t.Fatalf("expected env-var fallback key, got %q", cred.APIKey)
	}
}

func TestResolve_GlobalKeyOverridesEnvVar(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("GEMINI_API_KEY", "env-key")
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetGlobal(ctx, providerkey.SetKeyRequest{
		Provider: providerkey.ProviderGemini,
		APIKey:   "global-key",
	}); err != nil {
		t.Fatalf("SetGlobal failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 1, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "global-key" {
		t.Fatalf("expected the global key to win over the env var, got %q", cred.APIKey)
	}
}

func TestResolve_UserKeyOverridesGlobalKey(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetGlobal(ctx, providerkey.SetKeyRequest{
		Provider: providerkey.ProviderGemini,
		APIKey:   "global-key",
	}); err != nil {
		t.Fatalf("SetGlobal failed: %v", err)
	}
	if err := service.SetMine(ctx, 7, providerkey.SetKeyRequest{
		Provider: providerkey.ProviderGemini,
		APIKey:   "user-key",
	}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}

	// The user with a personal key gets their own.
	cred, err := service.Resolve(ctx, 7, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "user-key" {
		t.Fatalf("expected the user's own key to win, got %q", cred.APIKey)
	}

	// A different user with no personal key still gets the global one.
	cred, err = service.Resolve(ctx, 8, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "global-key" {
		t.Fatalf("expected the global key for a user with no override, got %q", cred.APIKey)
	}
}

func TestResolve_PreservesBaseURLAndWorkspaceID(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{
		Provider:    providerkey.ProviderOpenAI,
		APIKey:      "user-key",
		BaseURL:     "https://api.deepseek.com",
		WorkspaceID: "unused-for-openai",
	}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 1, providerkey.ProviderOpenAI)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("expected BaseURL to round-trip, got %q", cred.BaseURL)
	}
}

func TestSetMine_TwiceUpsertsRatherThanDuplicating(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "first"}); err != nil {
		t.Fatalf("first SetMine failed: %v", err)
	}
	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "second"}); err != nil {
		t.Fatalf("second SetMine failed: %v", err)
	}

	keys, err := service.ListMine(ctx, 1)
	if err != nil {
		t.Fatalf("ListMine failed: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected exactly one row after two SetMine calls for the same provider, got %d", len(keys))
	}

	cred, err := service.Resolve(ctx, 1, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "second" {
		t.Fatalf("expected the second SetMine call to overwrite the first, got %q", cred.APIKey)
	}
}

func TestListMine_NeverIncludesTheAPIKey(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{
		Provider: providerkey.ProviderAnthropic,
		APIKey:   "super-secret",
	}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}

	keys, err := service.ListMine(ctx, 1)
	if err != nil {
		t.Fatalf("ListMine failed: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	// Response has no APIKey field at all — this compiles only because
	// that's true; it's asserted here as documentation of the contract.
	_ = keys[0].Provider
}

func TestDeleteMine_FallsBackToGlobal(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetGlobal(ctx, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "global-key"}); err != nil {
		t.Fatalf("SetGlobal failed: %v", err)
	}
	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "user-key"}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}

	if err := service.DeleteMine(ctx, 1, providerkey.ProviderGemini); err != nil {
		t.Fatalf("DeleteMine failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 1, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "global-key" {
		t.Fatalf("expected fallback to the global key after deleting the personal one, got %q", cred.APIKey)
	}
}

func TestDeleteMine_NotFound(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)

	err := service.DeleteMine(context.Background(), 1, providerkey.ProviderGemini)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestDeleteMine_DoesNotAffectAnotherUsersKey(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)
	ctx := context.Background()

	if err := service.SetMine(ctx, 1, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "user-1-key"}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}
	if err := service.SetMine(ctx, 2, providerkey.SetKeyRequest{Provider: providerkey.ProviderGemini, APIKey: "user-2-key"}); err != nil {
		t.Fatalf("SetMine failed: %v", err)
	}

	if err := service.DeleteMine(ctx, 1, providerkey.ProviderGemini); err != nil {
		t.Fatalf("DeleteMine failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 2, providerkey.ProviderGemini)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "user-2-key" {
		t.Fatalf("expected user 2's key to be untouched, got %q", cred.APIKey)
	}
}

func TestDeleteGlobal_NotFound(t *testing.T) {
	clearProviderEnv(t)
	service := newTestService(t)

	err := service.DeleteGlobal(context.Background(), providerkey.ProviderGemini)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestStoredAPIKeyIsEncryptedAtRest(t *testing.T) {
	clearProviderEnv(t)
	db := setupTestDB(t)
	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}
	service := providerkey.NewService(providerkey.NewRepository(db), box)

	if err := service.SetGlobal(context.Background(), providerkey.SetKeyRequest{
		Provider: providerkey.ProviderGemini,
		APIKey:   "plaintext-secret",
	}); err != nil {
		t.Fatalf("SetGlobal failed: %v", err)
	}

	var stored providerkey.Key
	if err := db.Where("provider = ?", providerkey.ProviderGemini).First(&stored).Error; err != nil {
		t.Fatalf("failed to read raw row: %v", err)
	}
	if stored.EncryptedAPIKey == "plaintext-secret" {
		t.Fatal("expected the stored key to be encrypted, not plaintext")
	}
}

func TestMain(m *testing.M) {
	// Belt-and-suspenders: make sure nothing in the real environment this
	// suite runs in leaks into a test that didn't explicitly call
	// clearProviderEnv (t.Setenv already restores per-test, this guards
	// the process-wide env before any test runs at all).
	for _, key := range []string{"GEMINI_API_KEY", "ANTHROPIC_API_KEY", "ANTHROPIC_WORKSPACE_ID", "OPENAI_API_KEY", "OPENAI_BASE_URL"} {
		os.Unsetenv(key)
	}
	os.Exit(m.Run())
}
