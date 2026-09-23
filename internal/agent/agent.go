package agent

import (
	"context"
	"errors"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

// DefaultMaxToolIterations caps how many model round-trips a single Run
// call will make before giving up on a turn. Without a cap, a model that
// keeps requesting tools would loop indefinitely. Exported so callers
// building a system prompt (see chat.buildSystemPrompt) can tell the
// model its own real ceiling instead of a hardcoded, easily-stale number.
const DefaultMaxToolIterations = 10

type Agent struct {
	llm               ai_model.LLM
	registry          *tools.Registry
	model             string
	systemPrompt      string
	maxToolIterations int
}

func New(llm ai_model.LLM, registry *tools.Registry, model string, systemPrompt string, maxToolIterations int) *Agent {
	return &Agent{
		llm:               llm,
		registry:          registry,
		model:             model,
		systemPrompt:      systemPrompt,
		maxToolIterations: maxToolIterations,
	}
}

// Run behaves like calling Chat in a loop until the model stops requesting
// tools, but streams progress as it happens, across every round-trip in
// the loop: onChunk with each piece of assistant text, onToolCall the
// moment a call is about to run (before it can take seconds to finish),
// and onToolResult once that same call's result is in. All three may be
// nil if live output isn't needed — the returned message history has the
// full record either way.
func (a *Agent) Run(
	ctx context.Context,
	messages []ai_model.Message,
	onChunk func(text string),
	onToolCall func(call ai_model.ToolCall),
	onToolResult func(result ai_model.ToolResult),
) ([]ai_model.Message, error) {

	for iteration := 0; iteration < a.maxToolIterations; iteration++ {
		response, err := a.llm.StreamChat(ctx, ai_model.ChatRequest{
			Messages: messages,
			Model:    a.model,
			Tools:    a.registry.Definitions(),
			System:   a.systemPrompt,
		}, onChunk)

		if err != nil {
			// Callers (chat.Handler.SendMessage) only persist a turn's
			// history once Run returns without error — and messages
			// already includes the user's own new message, appended by
			// the caller before Run was ever called. Propagating err
			// as-is, even on this very first call, would silently drop
			// that message along with anything this turn already did (one
			// or more tool calls may have already run and be sitting in
			// messages). A transient provider hiccup (a proxy cutting a
			// long stream short, say) shouldn't cost the user their own
			// message and whatever real progress already happened —
			// closing the turn out with an explanatory note instead keeps
			// all of it in history.
			return append(messages, ai_model.Message{
				Role: ai_model.RoleAssistant,
				Content: fmt.Sprintf(
					"The model provider returned an error, so this turn is ending early: %v",
					err,
				),
			}), nil
		}

		if len(response.ToolCalls) == 0 {
			return append(messages, ai_model.Message{
				Role:    ai_model.RoleAssistant,
				Content: response.Content,
			}), nil
		}

		messages = append(messages, ai_model.Message{
			Role:      ai_model.RoleAssistant,
			ToolCalls: &response.ToolCalls,
		})

		for _, call := range response.ToolCalls {
			if onToolCall != nil {
				onToolCall(call)
			}
			result := a.executeTool(ctx, call)
			if onToolResult != nil {
				onToolResult(result)
			}
			messages = append(messages, ai_model.Message{
				Role:        ai_model.RoleTool,
				ToolResults: &[]ai_model.ToolResult{result},
			})
		}
	}

	return append(messages, ai_model.Message{
		Role: ai_model.RoleAssistant,
		Content: fmt.Sprintf(
			"Stopped after %d tool calls in a single turn to avoid an infinite loop.",
			a.maxToolIterations,
		),
	}), nil
}

func (a *Agent) executeTool(
	ctx context.Context,
	call ai_model.ToolCall,
) ai_model.ToolResult {

	if parseErr, ok := call.Metadata[ai_model.MetadataParseError].(string); ok {
		return ai_model.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error: fmt.Errorf(
				"the model's arguments for this call didn't parse as JSON (%s) — this usually means its response was cut off after hitting the output length limit; try asking for a smaller step",
				parseErr,
			),
		}
	}

	tool, ok := a.registry.Get(call.Name)

	if !ok {
		return ai_model.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error:      fmt.Errorf("unknown tool: %s", call.Name),
		}
	}

	result, err := tool.Execute(ctx, call.Arguments)

	if err != nil {
		return ai_model.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Error:      fmt.Errorf("tool execution failed: %w", err),
		}
	}

	// result.Data is the actual payload (a command's output, a file's
	// content, ...) — passed through as-is rather than wrapped in
	// result's own {data, success} struct, so callers downstream (the
	// SSE response, the stored history, the frontend) see the real
	// content instead of one more layer of JSON to unwrap. Success/failure
	// is carried by Error instead of by re-inspecting Data's shape.
	toolResult := ai_model.ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Result:     result.Data,
	}
	if !result.Success {
		toolResult.Error = errors.New(describeFailure(result.Data))
	}
	return toolResult
}

// describeFailure extracts a short, human-readable failure message from a
// failed tool's Data. Most tools return a plain string; the ones that also
// carry output alongside the failure (ssh_execute, run_shell_command) use
// {"output": ..., "error": ...} — pulling just "error" out keeps that
// message and the terminal output it's about visually separate instead of
// the same line.
func describeFailure(data any) string {
	switch v := data.(type) {
	case string:
		return v
	case map[string]any:
		if errText, ok := v["error"].(string); ok {
			return errText
		}
	}
	return fmt.Sprintf("%v", data)
}
