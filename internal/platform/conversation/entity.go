package conversation

import "github.com/Aliizi83/vohu/internal/platform/shared"


type Conversation struct {
	shared.BaseModel
	UserID   uint
	Title    string `gorm:"type:varchar(255)"`
	Provider string `gorm:"type:varchar(50);not null"`
	Model    string `gorm:"type:varchar(100);not null"`
	Archived bool `gorm:"default:false"`
	CustomModelID *uint
}

func (Conversation) TableName() string { return "conversations" }

type Message struct {
	shared.BaseModel
	ConversationID  uint
	Role            string `gorm:"type:varchar(20);not null"`
	Content         string `gorm:"type:text"`
	ToolCallsJSON   string `gorm:"type:text"`
	ToolResultsJSON string `gorm:"type:text"`
}

func (Message) TableName() string { return "conversation_messages" }

func init() {
	shared.RegisterModel(&Conversation{})
	shared.RegisterModel(&Message{})
}
