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

type failingTool struct{}

func (t *failingTool) Name() string        { return "failing" }
func (t *failingTool) Description() string { return "always fails, like a bad ssh_execute" }
func (t *failingTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{}
}
func (t *failingTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	return tools.ToolResult{
		Success: false,
		Data: map[string]any{
			"output": "command output\nwith multiple lines",
			"error":  "process exited with status 1",
		},
	}, nil
}

func TestRun_NoToolCalls_ReturnsFinalAssistantMessage(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{Content: "hello there"},
	}}
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model", "")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "hi"}}

	result, err := a.Run(context.Background(), messages, nil, nil, nil)
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
	a := New(llm, registry, "fake-model", "")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "do the thing"}}

	result, err := a.Run(context.Background(), messages, nil, nil, nil)
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

func TestRun_UnknownTool_ReturnsResultWithError(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{ToolCalls: []ai_model.ToolCall{{ID: "call-1", Name: "does_not_exist", Arguments: map[string]any{}}}},
		{Content: "ok"},
	}}
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model", "")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "x"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, m := range result {
		if m.Role != ai_model.RoleTool || m.ToolResults == nil {
			continue
		}
		for _, r := range *m.ToolResults {
			if r.Error != nil && r.Error.Error() == "unknown tool: does_not_exist" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("expected an 'unknown tool' error in history: %+v", result)
	}
}

// TestRun_ToolCallWithParseErrorMetadata_SkipsExecutionAndSurvivesTheTurn
// guards the actual bug reported live: a model's tool call whose
// arguments didn't parse as JSON (openai.go/anthropic.go now record this
// as Metadata instead of returning a hard error) must not crash the whole
// Run call with "unexpected end of JSON input" — it should short-circuit
// into a failed tool result (never touching noopTool.Execute) and let the
// turn continue normally.
func TestRun_ToolCallWithParseErrorMetadata_SkipsExecutionAndSurvivesTheTurn(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{ToolCalls: []ai_model.ToolCall{{
			ID:        "call-1",
			Name:      "noop",
			Arguments: map[string]any{},
			Metadata:  map[string]any{ai_model.MetadataParseError: "unexpected end of JSON input"},
		}}},
		{Content: "ok"},
	}}
	registry := tools.NewRegistry()
	tool := &noopTool{}
	registry.Register(tool)
	a := New(llm, registry, "fake-model", "")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "x"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool.executed != 0 {
		t.Fatalf("expected the tool to never run on malformed arguments, executed %d times", tool.executed)
	}

	var got *ai_model.ToolResult
	for _, m := range result {
		if m.Role != ai_model.RoleTool || m.ToolResults == nil {
			continue
		}
		for i := range *m.ToolResults {
			got = &(*m.ToolResults)[i]
		}
	}
	if got == nil || got.Error == nil {
		t.Fatalf("expected a failed tool result in history: %+v", result)
	}
}

func TestRun_ToolReportsFailure_SetsErrorAndKeepsOutputSeparate(t *testing.T) {
	llm := &stubLLM{responses: []ai_model.ChatResponse{
		{ToolCalls: []ai_model.ToolCall{{ID: "call-1", Name: "failing", Arguments: map[string]any{}}}},
		{Content: "ok"},
	}}
	registry := tools.NewRegistry()
	registry.Register(&failingTool{})
	a := New(llm, registry, "fake-model", "")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "x"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got *ai_model.ToolResult
	for _, m := range result {
		if m.Role != ai_model.RoleTool || m.ToolResults == nil {
			continue
		}
		for i := range *m.ToolResults {
			got = &(*m.ToolResults)[i]
		}
	}

	if got == nil {
		t.Fatalf("expected a tool result in history: %+v", result)
	}
	if got.Error == nil || got.Error.Error() != "process exited with status 1" {
		t.Fatalf("expected Error to carry just the failure line, got %+v", got.Error)
	}
	data, ok := got.Result.(map[string]any)
	if !ok || data["output"] != "command output\nwith multiple lines" {
		t.Fatalf("expected Result to still carry the raw output, got %+v", got.Result)
	}
}

func TestRun_StopsAtMaxToolIterations(t *testing.T) {
	llm := &infiniteLLM{}
	registry := tools.NewRegistry()
	registry.Register(&noopTool{})
	a := New(llm, registry, "fake-model", "")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "loop forever"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if llm.calls != DefaultMaxToolIterations {
		t.Fatalf("expected exactly %d model calls, got %d", DefaultMaxToolIterations, llm.calls)
	}

	last := result[len(result)-1]
	if last.Role != ai_model.RoleAssistant {
		t.Fatalf("expected final message to be from the assistant, got role %q", last.Role)
	}
}

// TestRun_LLMErrorOnFirstCall_ClosesTurnWithoutDiscardingTheUserMessage
// guards a real failure seen live: chat.Handler.SendMessage appends the
// user's new message to messages before ever calling Run, and only
// persists Run's returned history when Run returns no error at all. Run
// used to propagate a first-call provider error as-is, on the theory that
// "nothing happened yet this turn" — but the user's own message was
// already sitting in messages, so the caller silently dropped it, with
// nothing at all saved and no way to tell what happened without retyping
// the exact same message and hoping the transient provider hiccup didn't
// recur. Run must always return a message history (and no error) so
// there's a trail even when the very first model call fails.
func TestRun_LLMErrorOnFirstCall_ClosesTurnWithoutDiscardingTheUserMessage(t *testing.T) {
	llm := &stubLLM{} // no responses queued -> StreamChat returns an error immediately
	registry := tools.NewRegistry()
	a := New(llm, registry, "fake-model", "")

	messages := []ai_model.Message{{Role: ai_model.RoleUser, Content: "hi"}}

	result, err := a.Run(context.Background(), messages, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != len(messages)+1 {
		t.Fatalf("expected the user's message plus one explanatory assistant message, got %+v", result)
	}
	if result[0].Content != "hi" {
		t.Fatalf("expected the user's original message to survive, got %+v", result[0])
	}
	last := result[len(result)-1]
	if last.Role != ai_model.RoleAssistant || last.Content == "" {
		t.Fatalf("expected a final assistant message explaining the error, got %+v", last)
	}
}

// erroringAfterOneCallLLM answers its first StreamChat call with a tool
// call, then fails every call after that — simulating a provider that cuts
// a long stream short partway through a turn (a proxy dropping the
// connection mid-response, say), after real progress already happened.
type erroringAfterOneCallLLM struct{ calls int }

func (f *erroringAfterOneCallLLM) Chat(ctx context.Context, req ai_model.ChatRequest) (ai_model.ChatResponse, error) {
	return f.StreamChat(ctx, req, nil)
}

func (f *erroringAfterOneCallLLM) StreamChat(ctx context.Context, req ai_model.ChatRequest, onChunk func(string)) (ai_model.ChatResponse, error) {
	f.calls++
	if f.calls == 1 {
		return ai_model.ChatResponse{
			ToolCalls: []ai_model.ToolCall{{ID: "call-1", Name: "noop", Arguments: map[string]any{}}},
		}, nil
	}
	return ai_model.ChatResponse{}, fmt.Errorf("stream cut short")
}

// TestRun_LLMErrorAfterProgress_ClosesTurnInsteadOfDiscardingIt guards the
// live bug this was written for: a provider erroring out on a later
// call — after at least one tool call this turn already succeeded — used
// to make Run return that raw error and the caller (chat.Handler) would
// then persist nothing at all, silently losing the tool call and result
// that had already happened. It should instead close the turn out with an
// explanatory assistant message and no error, so everything done so far
// makes it into history.
func TestRun_LLMErrorAfterProgress_ClosesTurnInsteadOfDiscardingIt(t *testing.T) {
	llm := &erroringAfterOneCallLLM{}
	registry := tools.NewRegistry()
	tool := &noopTool{}
	registry.Register(tool)
	a := New(llm, registry, "fake-model", "")

	result, err := a.Run(context.Background(), []ai_model.Message{{Role: ai_model.RoleUser, Content: "x"}}, nil, nil, nil)
	if err != nil {
		t.Fatalf("expected no error once progress has been made, got %v", err)
	}
	if tool.executed != 1 {
		t.Fatalf("expected the tool call from the first response to have run, executed %d times", tool.executed)
	}

	sawToolResult := false
	for _, m := range result {
		if m.Role == ai_model.RoleTool && m.ToolResults != nil {
			sawToolResult = true
		}
	}
	if !sawToolResult {
		t.Fatalf("expected the earlier tool call/result to survive in history: %+v", result)
	}

	last := result[len(result)-1]
	if last.Role != ai_model.RoleAssistant || last.Content == "" {
		t.Fatalf("expected a final assistant message explaining the early stop, got %+v", last)
	}
}
