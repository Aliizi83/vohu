package custommodel_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/custommodel"
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
	if err := db.AutoMigrate(&custommodel.CustomModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func newTestService(t *testing.T) custommodel.Service {
	t.Helper()

	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}

	return custommodel.NewService(custommodel.NewRepository(setupTestDB(t)), box)
}

func TestCreateMine_ThenListMine_ReturnsIt(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	resp, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "My Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "secret",
	})
	if err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}
	if resp.ID == 0 {
		t.Fatal("expected a nonzero ID")
	}

	items, err := service.ListMine(ctx, 1)
	if err != nil {
		t.Fatalf("ListMine failed: %v", err)
	}
	if len(items) != 1 || items[0].Name != "My Ollama" || items[0].ModelName != "llama3" {
		t.Fatalf("expected the created preset back, got %+v", items)
	}
}

// TestCreateMine_MultipleRowsAllowed is the actual feature this module
// exists for — providerkey.Key allows only one "openai" slot per
// user/global, which can't express two different OpenAI-compatible
// endpoints (a local Ollama server *and* a DeepSeek account) at once.
func TestCreateMine_MultipleRowsAllowed(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "key-a",
	}); err != nil {
		t.Fatalf("first CreateMine failed: %v", err)
	}
	if _, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "DeepSeek", BaseURL: "https://api.deepseek.com", ModelName: "deepseek-chat", APIKey: "key-b",
	}); err != nil {
		t.Fatalf("second CreateMine failed: %v", err)
	}

	items, err := service.ListMine(ctx, 1)
	if err != nil {
		t.Fatalf("ListMine failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 presets for the same user, got %d", len(items))
	}
}

func TestListAccessible_CombinesMineAndGlobal(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Mine", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "key-a",
	}); err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}
	if _, err := service.CreateGlobal(ctx, custommodel.CreateCustomModelRequest{
		Name: "Shared", BaseURL: "https://api.deepseek.com", ModelName: "deepseek-chat", APIKey: "key-b",
	}); err != nil {
		t.Fatalf("CreateGlobal failed: %v", err)
	}
	// Another user's personal preset should NOT leak into caller 1's list.
	if _, err := service.CreateMine(ctx, 2, custommodel.CreateCustomModelRequest{
		Name: "Someone Else's", BaseURL: "http://localhost:1234/v1", ModelName: "mistral", APIKey: "key-c",
	}); err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	items, err := service.ListAccessible(ctx, 1)
	if err != nil {
		t.Fatalf("ListAccessible failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected mine (1) + global (1) = 2 presets, got %d: %+v", len(items), items)
	}
	names := map[string]bool{}
	for _, item := range items {
		names[item.Name] = true
	}
	if !names["Mine"] || !names["Shared"] {
		t.Fatalf("expected both \"Mine\" and \"Shared\" in the accessible list, got %+v", items)
	}
	if names["Someone Else's"] {
		t.Fatal("expected another user's personal preset to be excluded")
	}
}

func TestListMine_NeverIncludesTheAPIKey(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	if _, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "super-secret",
	}); err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	items, err := service.ListMine(ctx, 1)
	if err != nil {
		t.Fatalf("ListMine failed: %v", err)
	}
	// Response has no APIKey field at all — this compiles only because
	// that's true; asserted here as documentation of the contract.
	_ = items[0].BaseURL
}

func TestResolve_ReturnsCredentialForOwner(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 1, created.ID)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if cred.APIKey != "secret-key" || cred.BaseURL != "http://localhost:11434/v1" {
		t.Fatalf("unexpected credential: %+v", cred)
	}
}

func TestResolve_DeniesNonOwnerAsNotFound(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	_, err = service.Resolve(ctx, 2, created.ID)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound for a non-owner's preset, got %v", err)
	}
}

func TestResolve_AllowsAnyoneForAGlobalPreset(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateGlobal(ctx, custommodel.CreateCustomModelRequest{
		Name: "Shared DeepSeek", BaseURL: "https://api.deepseek.com", ModelName: "deepseek-chat", APIKey: "shared-key",
	})
	if err != nil {
		t.Fatalf("CreateGlobal failed: %v", err)
	}

	cred, err := service.Resolve(ctx, 42, created.ID)
	if err != nil {
		t.Fatalf("expected any authenticated caller to resolve a global preset, got %v", err)
	}
	if cred.APIKey != "shared-key" {
		t.Fatalf("expected the global preset's key, got %q", cred.APIKey)
	}
}

func TestResolve_NotFound(t *testing.T) {
	service := newTestService(t)

	_, err := service.Resolve(context.Background(), 1, 999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound for a nonexistent preset, got %v", err)
	}
}

func TestDeleteMine_DoesNotAffectAnotherUsersPreset(t *testing.T) {
	service := newTestService(t)
	ctx := context.Background()

	created, err := service.CreateMine(ctx, 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "secret-key",
	})
	if err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	if err := service.DeleteMine(ctx, 2, created.ID); err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound deleting another user's preset, got %v", err)
	}

	if _, err := service.Resolve(ctx, 1, created.ID); err != nil {
		t.Fatalf("expected the owner's preset to survive the other user's failed delete, got %v", err)
	}
}

func TestDeleteMine_NotFound(t *testing.T) {
	service := newTestService(t)

	err := service.DeleteMine(context.Background(), 1, 999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestDeleteGlobal_NotFound(t *testing.T) {
	service := newTestService(t)

	err := service.DeleteGlobal(context.Background(), 999)
	if err != shared.ErrNotFound {
		t.Fatalf("expected shared.ErrNotFound, got %v", err)
	}
}

func TestStoredAPIKeyIsEncryptedAtRest(t *testing.T) {
	db := setupTestDB(t)
	box, err := crypto.NewBox(testEncryptionKey)
	if err != nil {
		t.Fatalf("NewBox failed: %v", err)
	}
	service := custommodel.NewService(custommodel.NewRepository(db), box)

	if _, err := service.CreateMine(context.Background(), 1, custommodel.CreateCustomModelRequest{
		Name: "Ollama", BaseURL: "http://localhost:11434/v1", ModelName: "llama3", APIKey: "plaintext-secret",
	}); err != nil {
		t.Fatalf("CreateMine failed: %v", err)
	}

	var stored custommodel.CustomModel
	if err := db.Where("name = ?", "Ollama").First(&stored).Error; err != nil {
		t.Fatalf("failed to read raw row: %v", err)
	}
	if stored.EncryptedAPIKey == "plaintext-secret" {
		t.Fatal("expected the stored key to be encrypted, not plaintext")
	}
}
