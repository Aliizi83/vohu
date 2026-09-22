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
	accumulateOpenAIMessage(&response, message)

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected exactly 1 tool call, got %d", len(response.ToolCalls))
	}
	if response.ToolCalls[0].Name != "list_ssh_connections" {
		t.Fatalf("unexpected tool call name: %+v", response.ToolCalls[0])
	}
	if len(response.ToolCalls[0].Arguments) != 0 {
		t.Fatalf("expected an empty arguments map, got %+v", response.ToolCalls[0].Arguments)
	}
	if response.ToolCalls[0].Metadata != nil {
		t.Fatalf("expected no parseError metadata for a merely-empty argument string, got %+v", response.ToolCalls[0].Metadata)
	}
}

func TestAccumulateOpenAIMessage_WhitespaceOnlyArgumentsIsTreatedAsEmptyObject(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{ID: "call-1", Function: openai.ChatCompletionMessageToolCallFunction{Name: "list_ssh_connections", Arguments: "   "}},
		},
	}

	var response ai_model.ChatResponse
	accumulateOpenAIMessage(&response, message)

	if response.ToolCalls[0].Metadata != nil {
		t.Fatalf("expected no parseError metadata for whitespace-only arguments, got %+v", response.ToolCalls[0].Metadata)
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
	accumulateOpenAIMessage(&response, message)

	if response.ToolCalls[0].Arguments["program"] != "ls" {
		t.Fatalf("expected real arguments to still parse correctly, got %+v", response.ToolCalls[0].Arguments)
	}
	if response.ToolCalls[0].Metadata != nil {
		t.Fatalf("expected no parseError metadata for valid arguments, got %+v", response.ToolCalls[0].Metadata)
	}
}

// TestAccumulateOpenAIMessage_TrulyInvalidArgumentsIsRecordedNotFatal
// guards the bug this test used to pin down the wrong way: truly
// malformed JSON (most often the model's own output getting cut off
// mid-argument, e.g. a large generated file hitting the provider's
// max-output-tokens limit) used to make this function return an error,
// which aborted the entire Chat/StreamChat call — and with it the whole
// turn, with nothing persisted and the user seeing a bare "unexpected end
// of JSON input". It's now recorded on the call itself instead, so
// agent.Run can turn it into an ordinary failed tool result and the turn
// survives.
func TestAccumulateOpenAIMessage_TrulyInvalidArgumentsIsRecordedNotFatal(t *testing.T) {
	message := openai.ChatCompletionMessage{
		ToolCalls: []openai.ChatCompletionMessageToolCall{
			{ID: "call-1", Function: openai.ChatCompletionMessageToolCallFunction{Name: "ssh_execute", Arguments: "{not json"}},
		},
	}

	var response ai_model.ChatResponse
	accumulateOpenAIMessage(&response, message)

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected the call to still show up in ToolCalls, got %+v", response.ToolCalls)
	}
	if len(response.ToolCalls[0].Arguments) != 0 {
		t.Fatalf("expected an empty arguments map for unparseable input, got %+v", response.ToolCalls[0].Arguments)
	}
	if _, ok := response.ToolCalls[0].Metadata[ai_model.MetadataParseError].(string); !ok {
		t.Fatalf("expected parseError metadata to be set, got %+v", response.ToolCalls[0].Metadata)
	}
}
