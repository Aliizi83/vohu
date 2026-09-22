package chat_settings

import "github.com/Aliizi83/vohu/internal/platform/shared"

type ChatSetting struct {
	shared.BaseModel
	ConversationID     uint   `gorm:"uniqueIndex;not null"`
	MaxToolIntegration uint   `gorm:"not null;default:10"`
	DefaultPrompt      string `gorm:"type:text;default:null"`
}

func (ChatSetting) TableName() string { return "chat_settings" }

func init() {
	shared.RegisterModel(&ChatSetting{})
}
