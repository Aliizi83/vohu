package ai_model

import "context"

type LLM interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)
}

type ChatRequest struct {
	Messages []Message
}

type Message struct {
	Role    string
	Content string
}

type ChatResponse struct {
	Content   string
	ToolCalls []ToolCall
	// Usage     Usage
}

type ToolCall struct {
	Name      string
	Arguments map[string]any
}
