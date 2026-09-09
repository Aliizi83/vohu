package conversation

type CreateConversationRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Provider string `json:"provider" binding:"required"`
	Model    string `json:"model" binding:"required"`

	// CustomModelID, when set, names one of the caller's (or a global)
	// custommodel.CustomModel presets to use for this conversation's
	// credentials instead of the single per-user/global "openai" key —
	// see Conversation.CustomModelID. Not validated here (conversation
	// never imports custommodel); chat.buildLLM resolves and checks
	// ownership lazily the first time the conversation is actually used.
	CustomModelID *uint `json:"customModelId"`
}

type Response struct {
	ID            uint   `json:"id"`
	Title         string `json:"title"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	CustomModelID *uint  `json:"customModelId,omitempty"`
}

// ToResponse is exported (unlike other modules' toResponse) because this
// module has no HTTP handler of its own — the chat module owns every
// conversation-related route and needs this mapping to build its own
// responses.
func ToResponse(c Conversation) Response {
	return Response{
		ID:            c.ID,
		Title:         c.Title,
		Provider:      c.Provider,
		Model:         c.Model,
		CustomModelID: c.CustomModelID,
	}
}
