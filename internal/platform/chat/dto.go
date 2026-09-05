package chat

import "github.com/Aliizi83/vohu/internal/ai_model"

type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// ToolResultResponse is MessageResponse's JSON-safe stand-in for
// ai_model.ToolResult — same reasoning as conversation's storedToolResult:
// ToolResult.Error is the error interface, whose concrete types have no
// exported fields for json.Marshal to see, so it's flattened to a string
// here too. This is a separate type from conversation's rather than a
// shared one — one is a DB row shape, this is an HTTP response shape, and
// nothing but the shared translation logic itself between them.
type ToolResultResponse struct {
	ToolCallID string `json:"toolCallId"`
	Name       string `json:"name"`
	Result     any    `json:"result"`
	Error      string `json:"error,omitempty"`
}

type MessageResponse struct {
	Role        string               `json:"role"`
	Content     string               `json:"content,omitempty"`
	ToolCalls   []ai_model.ToolCall  `json:"toolCalls,omitempty"`
	ToolResults []ToolResultResponse `json:"toolResults,omitempty"`
}

func toMessageResponse(msg ai_model.Message) MessageResponse {
	resp := MessageResponse{
		Role:    string(msg.Role),
		Content: msg.Content,
	}

	if msg.ToolCalls != nil {
		resp.ToolCalls = *msg.ToolCalls
	}

	if msg.ToolResults != nil {
		resp.ToolResults = make([]ToolResultResponse, 0, len(*msg.ToolResults))
		for _, result := range *msg.ToolResults {
			entry := ToolResultResponse{
				ToolCallID: result.ToolCallID,
				Name:       result.Name,
				Result:     result.Result,
			}
			if result.Error != nil {
				entry.Error = result.Error.Error()
			}
			resp.ToolResults = append(resp.ToolResults, entry)
		}
	}

	return resp
}
