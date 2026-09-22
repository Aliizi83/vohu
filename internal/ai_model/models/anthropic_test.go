package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/anthropics/anthropic-sdk-go"
)

// toolUseBlock builds a ContentBlockUnion the same way the SDK does when
// decoding one off the wire — ContentBlockUnion.AsToolUse() re-parses from
// the raw bytes captured at unmarshal time, so setting the struct's fields
// directly (skipping json.Unmarshal) would leave that raw copy empty and
// AsToolUse() would silently return a zero value. inputJSON, when
// non-empty, must itself be a syntactically valid JSON value (an object,
// a string, ...) since it's spliced into a real JSON document — Anthropic
// never actually sends a syntactically-broken document over the wire, so
// what accumulateAnthropicResponse must tolerate is a well-formed
// "input" whose value merely isn't an object (a JSON string, say), which
// exercises the same json.Unmarshal-into-map failure path a truncated
// stream would.
func toolUseBlock(t *testing.T, id, name string, inputJSON string) anthropic.ContentBlockUnion {
	t.Helper()
	inputField := ""
	if inputJSON != "" {
		inputField = fmt.Sprintf(`,"input":%s`, inputJSON)
	}
	raw := fmt.Sprintf(`{"type":"tool_use","id":%q,"name":%q%s}`, id, name, inputField)
	var block anthropic.ContentBlockUnion
	if err := json.Unmarshal([]byte(raw), &block); err != nil {
		t.Fatalf("failed to build test ContentBlockUnion: %v", err)
	}
	return block
}

func TestAccumulateAnthropicResponse_EmptyInputIsTreatedAsEmptyObject(t *testing.T) {
	var response ai_model.ChatResponse
	accumulateAnthropicResponse(&response, []anthropic.ContentBlockUnion{
		toolUseBlock(t, "call-1", "list_ssh_connections", ""),
	})

	if len(response.ToolCalls) != 1 {
		t.Fatalf("expected exactly 1 tool call, got %d", len(response.ToolCalls))
	}
	if len(response.ToolCalls[0].Arguments) != 0 {
		t.Fatalf("expected an empty arguments map, got %+v", response.ToolCalls[0].Arguments)
	}
	if response.ToolCalls[0].Metadata != nil {
		t.Fatalf("expected no parseError metadata for empty input, got %+v", response.ToolCalls[0].Metadata)
	}
}

func TestAccumulateAnthropicResponse_RealArgumentsStillParseNormally(t *testing.T) {
	var response ai_model.ChatResponse
	accumulateAnthropicResponse(&response, []anthropic.ContentBlockUnion{
		toolUseBlock(t, "call-1", "ssh_execute", `{"connectionId":5,"program":"ls"}`),
	})

	if response.ToolCalls[0].Arguments["program"] != "ls" {
		t.Fatalf("expected real arguments to still parse correctly, got %+v", response.ToolCalls[0].Arguments)
	}
	if response.ToolCalls[0].Metadata != nil {
		t.Fatalf("expected no parseError metadata for valid arguments, got %+v", response.ToolCalls[0].Metadata)
	}
}

// TestAccumulateAnthropicResponse_TrulyInvalidInputIsRecordedNotFatal
// mirrors the OpenAI-side test: input that parses as JSON but isn't the
// object the tool call needs (a plain string, the shape a truncated
// stream can leave behind) must not make this function return an error
// and abort the whole turn — it gets recorded on the call so agent.Run
// can surface it as an ordinary failed tool result instead.
func TestAccumulateAnthropicResponse_TrulyInvalidInputIsRecordedNotFatal(t *testing.T) {
	var response ai_model.ChatResponse
	accumulateAnthropicResponse(&response, []anthropic.ContentBlockUnion{
		toolUseBlock(t, "call-1", "ssh_execute", `"not an object"`),
	})

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
