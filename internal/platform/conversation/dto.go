package conversation

type CreateConversationRequest struct {
	Title    string `json:"title" binding:"required,max=255"`
	Provider string `json:"provider" binding:"required"`
	Model    string `json:"model" binding:"required"`
}

type Response struct {
	ID       uint   `json:"id"`
	Title    string `json:"title"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ToResponse is exported (unlike other modules' toResponse) because this
// module has no HTTP handler of its own — the chat module owns every
// conversation-related route and needs this mapping to build its own
// responses.
func ToResponse(c Conversation) Response {
	return Response{
		ID:       c.ID,
		Title:    c.Title,
		Provider: c.Provider,
		Model:    c.Model,
	}
}
