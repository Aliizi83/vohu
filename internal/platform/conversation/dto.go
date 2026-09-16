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

// UpdateConversationRequest doubles as rename, archive/unarchive, and
// switching which model this conversation talks to going forward — an
// empty Title means "don't change" (same convention as
// sshconn.UpdateSSHConnectionRequest), Archived is a pointer since its
// zero value (false) has to be distinguishable from "not provided."
//
// Provider/Model/CustomModelID travel together as one unit, the same way
// CreateConversationRequest treats them: a non-empty Provider is what
// triggers the switch, and CustomModelID is replaced wholesale (nil
// clears it) rather than patched independently — switching from a custom
// preset back to a built-in model is then just "send the built-in
// provider/model and omit customModelId," not a separate call. Existing
// messages stay in history regardless of which model produced them; only
// turns from this point on use the new one.
type UpdateConversationRequest struct {
	Title         string `json:"title" binding:"omitempty,max=255"`
	Archived      *bool  `json:"archived"`
	Provider      string `json:"provider" binding:"omitempty"`
	Model         string `json:"model" binding:"required_with=Provider"`
	CustomModelID *uint  `json:"customModelId"`
}

type Response struct {
	ID            uint   `json:"id"`
	Title         string `json:"title"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	CustomModelID *uint  `json:"customModelId,omitempty"`
	Archived      bool   `json:"archived"`
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
		Archived:      c.Archived,
	}
}
