package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

// stubLLM lets each test script canned StreamChat responses in order.
type stubLLM struct {
	responses []ai_model.ChatResponse
	calls     int
}

func (s *stubLLM) Chat(ctx context.Context, req ai_model.ChatRequest) (ai_model.ChatResponse, error) {
	return s.StreamChat(ctx, req, nil)
}

func (s *stubLLM) StreamChat(ctx context.Context, req ai_model.ChatRequest, onChunk func(string)) (ai_model.ChatResponse, error) {
	if s.calls >= len(s.responses) {
		return ai_model.ChatResponse{}, fmt.Errorf("stubLLM: no response queued for call %d", s.calls)
	}
	response := s.responses[s.calls]
	s.calls++
	if onChunk != nil && response.Content != "" {
		onChunk(response.Content)
	}
	return response, nil
}

// infiniteLLM always requests the same tool call — used to verify the
// iteration cap actually stops the loop.
type infiniteLLM struct {
	calls int
}

func (f *infiniteLLM) Chat(ctx context.Context, req ai_model.ChatRequest) (ai_model.ChatResponse, error) {
	return f.StreamChat(ctx, req, nil)
}

func (f *infiniteLLM) StreamChat(ctx context.Context, req ai_model.ChatRequest, onChunk func(string)) (ai_model.ChatResponse, error) {
	f.calls++
	return ai_model.ChatResponse{
		ToolCalls: []ai_model.ToolCall{
			{ID: fmt.Sprintf("call-%d", f.calls), Name: "noop", Arguments: map[string]any{}},
		},
	}, nil
}

type noopTool struct{ executed int }

func (t *noopTool) Name() string        { return "noop" }
func (t *noopTool) Description() string { return "does nothing" }
func (t *noopTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{}
}
func (t *noopTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	t.executed++
	return tools.ToolResult{Success: true, Data: "ok"}, nil
}

func TestRun_NoToolCalls_ReturnsFinalAssistantMessage(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{Content: "hello there"},
	}}
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "hi"}}

	result, err := a.Run(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 messages (user + assistant), got %d", len(result))
	}

	last := result[len(result)-1]
	if last.Role != ai_model.RoleAssistant || last.Content != "hello there" {
		t.Fatalf("unexpected final message: %+v", last)
	}
}

func TestRun_ExecutesToolCallsAndAggregatesResults(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{ToolCalls: []ai_model.ToolCall{{ID: "call-1", Name: "noop", Arguments: map[string]any{}}}},
		{Content: "done"},
	}}
	registry := tools.NewRegistry()
	tool := &noopTool{}
	registry.Register(tool)
	a := New(llm, registry, "fake-model")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "do the thing"}}

	result, err := a.Run(context.Background(), messages, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tool.executed != 1 {
		t.Fatalf("expected tool to execute once, executed %d times", tool.executed)
	}

	var sawToolResult bool
	for _, m := range result {
		if m.Role == ai_model.RoleTool && m.ToolResults != nil {
			sawToolResult = true
		}
	}
	if !sawToolResult {
		t.Fatalf("expected a tool-role message with results in history: %+v", result)
	}

	last := result[len(result)-1]
	if last.Content != "done" {
		t.Fatalf("expected final assistant content %q, got %q", "done", last.Content)
	}
}

func TestRun_UnknownTool_ReturnsResultWithoutError(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{ToolCalls: []ai_model.ToolCall{{ID: "call-1", Name: "does_not_exist", Arguments: map[string]any{}}}},
		{Content: "ok"},
	}}
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "x"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, m := range result {
		if m.Role != ai_model.RoleTool || m.ToolResults == nil {
			continue
		}
		for _, r := range *m.ToolResults {
			if r.Result == "Unknown tool: does_not_exist" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected an 'Unknown tool' result in history: %+v", result)
	}
}

func TestRun_StopsAtMaxToolIterations(t *testing.T) {
	llm := &infiniteLLM{}
	registry := tools.NewRegistry()
	registry.Register(&noopTool{})
	a := New(llm, registry, "fake-model")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "loop forever"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if llm.calls != defaultMaxToolIterations {
		t.Fatalf("expected exactly %d model calls, got %d", defaultMaxToolIterations, llm.calls)
	}

	last := result[len(result)-1]
	if last.Role != ai_model.RoleAssistant {
		t.Fatalf("expected final message to be from the assistant, got role %q", last.Role)
	}
}

func TestRun_PropagatesLLMError(t *testing.T) {
	llm := &stubLLM{} // no responses queued -> StreamChat returns an error immediately
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "hi"}}

	result, err := a.Run(context.Background(), messages, nil)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(result) != len(messages) {
		t.Fatalf("expected messages to be returned unmodified on error, got %+v", result)
	}
}
