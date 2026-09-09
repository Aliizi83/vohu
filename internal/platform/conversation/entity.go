package conversation

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Conversation is one chat thread — a fixed Provider/Model chosen once at
// creation (no per-message provider switching), mirroring how cmd/vohu's
// chooseModel prompt works today.
type Conversation struct {
	shared.BaseModel
	UserID   uint
	Title    string `gorm:"type:varchar(255)"`
	Provider string `gorm:"type:varchar(50);not null"`
	Model    string `gorm:"type:varchar(100);not null"`

	// CustomModelID is set when Provider is "openai" and the caller picked
	// one of their (or a global) custommodel.CustomModel presets rather
	// than the single per-user/global "openai" providerkey.Key slot — nil
	// for every other conversation. Kept as a plain nullable ID, not a
	// foreign key, same decoupling rule every cross-module reference in
	// this codebase already follows (no import of custommodel here);
	// chat.buildLLM is what actually resolves it.
	CustomModelID *uint
}

func (Conversation) TableName() string { return "conversations" }

// Message persists one internal/ai_model.Message. ToolCalls/ToolResults
// are nested structs on ai_model.Message, not flat columns, so they're
// stored as JSON text — see codec.go for the conversion both ways.
// Ordering within a conversation is by ID (an auto-increment primary key
// is strictly increasing on insert, so no separate sequence column is
// needed).
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
