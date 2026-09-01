package agent

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

type Agent struct {
	llm      ai_model.LLM
	registry *tools.Registry
	model    string
}

func New(llm ai_model.LLM, registry *tools.Registry, model string) *Agent {
	return &Agent{
		llm:      llm,
		registry: registry,
		model:    model,
	}
}

// Run behaves like calling Chat in a loop until the model stops requesting
// tools, but streams the assistant's text: onChunk is called with each
// piece of text as it arrives, across every round-trip in the loop. Pass
// a no-op function if live output isn't needed.
func (a *Agent) Run(
	ctx context.Context,
	messages []ai_model.Message,
	onChunk func(text string),
) ([]ai_model.Message, error) {

	for {
		response, err := a.llm.StreamChat(ctx, ai_model.ChatRequest{
			Messages: messages,
			Model:    a.model,
			Tools:    a.registry.Definitions(),
		}, onChunk)

		if err != nil {
			return messages, err
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
			messages = append(messages, ai_model.Message{
				Role:        ai_model.RoleTool,
				ToolResults: &[]ai_model.ToolResult{a.executeTool(ctx, call)},
			})
		}
	}
}

func (a *Agent) executeTool(
	ctx context.Context,
	call ai_model.ToolCall,
) ai_model.ToolResult {

	tool, ok := a.registry.Get(call.Name)

	if !ok {
		return ai_model.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Result:     fmt.Sprintf("Unknown tool: %s", call.Name),
		}
	}

	result, err := tool.Execute(ctx, call.Arguments)

	if err != nil {
		return ai_model.ToolResult{
			ToolCallID: call.ID,
			Name:       call.Name,
			Result:     fmt.Sprintf("Tool execution failed: %v", err),
		}
	}

	return ai_model.ToolResult{
		ToolCallID: call.ID,
		Name:       call.Name,
		Result:     result,
	}
}
