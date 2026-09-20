package models

import (
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/openai/openai-go"
)

// TestAccumulateOpenAIMessage_EmptyArgumentsIsTreatedAsEmptyObject guards
// a real failure seen against a live OpenAI-compatible proxy: a
// zero-parameter tool call (list_ssh_connections, say) came back with
// Arguments == "" instead of "{}" — json.Unmarshal on an empty string
// always fails with "unexpected end of JSON input", which isn't a
// problem with the call, just an empty object spelled differently.
func TestAccumulateOpenAIMessage_EmptyArgumentsIsTreatedAsEmptyObject(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{
				ID: "call-1",
				Function: openai.ChatCompletionMessageToolCallFunction{
					Name:      "list_ssh_connections",
					Arguments: "",
				},
			},
		},
	}

	var response ai_model.ChatResponse
	if err := accumulateOpenAIMessage(&response, message); err != nil {
		t.Fatalf("expected no error for an empty-string arguments field, got %v", err)
	}

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected exactly 1 tool call, got %d", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "list_ssh_connections" {
		t.Fatalf("unexpected tool call name: %+v", response.ToolCalls[0])
	}
	if len(response.ToolCalls[0].Arguments) != 0 {
		t.Fatalf("expected an empty arguments map, got %+v", response.ToolCalls[0].Arguments)
	}
}

func TestAccumulateOpenAIMessage_WhitespaceOnlyArgumentsIsTreatedAsEmptyObject(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{ID: "call-1", Function: openai.ChatCompletionMessageToolCallFunction{Name: "list_ssh_connections", Arguments: "   "}},
		},
	}

	var response ai_model.ChatResponse
	if err := accumulateOpenAIMessage(&response, message); err != nil {
		t.Fatalf("expected no error for whitespace-only arguments, got %v", err)
	}
}

func TestAccumulateOpenAIMessage_RealArgumentsStillParseNormally(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{
				ID: "call-1",
				Function: openai.ChatCompletionMessageToolCallFunction{
					Name:      "ssh_execute",
					Arguments: `{"connectionId":5,"program":"ls"}`,
				},
			},
		},
	}

	var response ai_model.ChatResponse
	if err := accumulateOpenAIMessage(&response, message); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if response.ToolCalls[0].Arguments["program"] != "ls" {
		t.Fatalf("expected real arguments to still parse correctly, got %+v", response.ToolCalls[0].Arguments)
	}
}

func TestAccumulateOpenAIMessage_TrulyInvalidArgumentsStillFails(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{ID: "call-1", Function: openai.ChatCompletionMessageToolCallFunction{Name: "ssh_execute", Arguments: "{not json"}},
		},
	}

	var response ai_model.ChatResponse
	if err := accumulateOpenAIMessage(&response, message); err == nil {
		t.Fatal("expected genuinely malformed JSON to still be reported as an error")
	}
}
