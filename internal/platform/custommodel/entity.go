package custommodel

import "github.com/Aliizi83/vohu/internal/platform/shared"

// CustomModel is one named OpenAI-compatible endpoint a caller can pick
// when starting a conversation — unlike providerkey.Key (one slot per
// provider per user/global), there can be any number of these, which is
// the whole point: a user with both a local Ollama server and a DeepSeek
// account needs two different (BaseURL, APIKey) pairs under the single
// "openai" provider, not one. UserID nil means a global preset any
// authenticated user can pick, same convention providerkey.Key uses.
type CustomModel struct {
	shared.BaseModel
	UserID          *uint
	Name            string `gorm:"type:varchar(100);not null"`
	BaseURL         string `gorm:"type:varchar(255);not null"`
	ModelName       string `gorm:"type:varchar(255);not null"`
	EncryptedAPIKey string `gorm:"type:text;not null"`
}

func (CustomModel) TableName() string { return "custom_models" }

func init() {
	shared.RegisterModel(&CustomModel{})
}
