package providerkey

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Provider identifies which LLM provider a key is for — the same three
// strings internal/platform/chat's buildLLM already switches on.
type Provider string

const (
	ProviderGemini    Provider = "gemini"
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
)

// Key is one credential row for one provider. UserID nil means this is
// the global/default key an admin configured for everyone; a non-nil
// UserID is one user's personal override for that provider. There's no
// DB-level uniqueness constraint on (UserID, Provider) — Postgres treats
// multiple NULLs as distinct under a plain unique index, so enforcing "at
// most one" is done in the repository the same way rbac's
// UpsertResourcePermission already does it: find-then-update-or-create.
//
// Only APIKey is a secret (AES-GCM encrypted via pkg/crypto, same box
// sshconn uses for SSH credentials) — BaseURL and WorkspaceID are plain
// configuration, not secrets, so they're stored as-is.
type Key struct {
	shared.BaseModel
	UserID          *uint
	Provider        Provider `gorm:"type:varchar(20);not null"`
	EncryptedAPIKey string   `gorm:"type:text;not null"`
	BaseURL         string   `gorm:"type:varchar(255)"`
	WorkspaceID     string   `gorm:"type:varchar(255)"`
}

func (Key) TableName() string { return "provider_keys" }

func init() {
	shared.RegisterModel(&Key{})
}
