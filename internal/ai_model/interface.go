package ai_model

import "context"

type LLM interface {
	Chat(ctx context.Context, request ChatRequest) (ChatResponse, error)

	// StreamChat behaves like Chat, but calls onChunk with each piece of
	// assistant text as it arrives instead of only returning the final
	// response once the model is done. onChunk is never called with tool
	// call data — the returned ChatResponse still carries the complete,
	// assembled ToolCalls once streaming finishes.
	StreamChat(ctx context.Context, request ChatRequest, onChunk func(text string)) (ChatResponse, error)
}

type ChatRequest struct {
	Messages []Message
	Model    string
	Tools    []ToolDefinition
	// System is a system-role prompt sent ahead of Messages — instructions
	// and context the model should follow for the whole conversation,
	// distinct from any of the three message roles below.
	System string
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
	Parameters  ToolParameters
}

type ToolParameters struct {
	Properties map[string]ToolProperty
	Required   []string
}

type ToolProperty struct {
	Type        string
	Description string
	Items       *ToolProperty
}

type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
	Metadata  map[string]any `json:"metadata"`
}

// MetadataParseError is the Metadata key a provider sets when a model's
// tool call arrived with arguments that don't parse as JSON — usually the
// model's own output got cut off mid-argument (a large generated file
// hitting the provider's max-output-tokens limit) rather than a real bug
// in the call. Arguments is left as an empty map in that case; agent.Run
// checks this key before ever handing Arguments to the tool, so a
// malformed call surfaces as an ordinary failed tool result instead of
// aborting the whole turn.
const MetadataParseError = "parseError"

type ToolResult struct {
	ToolCallID string
	Name       string
	Result     any
	Error      error
}
