package ai_model

import "context"

type LLM interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Messages []Message
	Model    string
	Tools    []ToolDefinition
}

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role        Role
	Content     string
	ToolCalls   *[]ToolCall
	ToolResults *[]ToolResult
}

type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	// Usage     Usage
}

type ToolDefinition struct {
	Name        string
	Description string
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Metadata  map[string]any `json:"metadata"`
}

type ToolResult struct {
	ToolCallID string
	Name       string
	Result     any
	Error      error
}
