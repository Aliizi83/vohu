package chat_settings

type Response struct {
	ConversationID     uint   `json:"conversationId"`
	MaxToolIntegration uint   `json:"maxToolIntegration"`
	DefaultPrompt      string `json:"defaultPrompt"`
}

func ToResponse(s ChatSetting) Response {
	return Response{
		ConversationID:     s.ConversationID,
		MaxToolIntegration: s.MaxToolIntegration,
		DefaultPrompt:      s.DefaultPrompt,
	}
}
