package models

import (
	"context"
	"encoding/json"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

const (
	ClaudeOpus5   ModelName = "claude-opus-5"
	ClaudeSonnet5 ModelName = "claude-sonnet-5"
	ClaudeHaiku45 ModelName = "claude-haiku-4-5-20251001"
)

const anthropicMaxTokens = 16000

type AnthropicAgent struct {
	ApiKey string
}

func NewAnthropicAgent(apiKey string) *AnthropicAgent {
	return &AnthropicAgent{
		ApiKey: apiKey,
	}
}

func (agent *AnthropicAgent) Chat(
	ctx context.Context,
	request ai_model.ChatRequest,
) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client := anthropic.NewClient(
		option.WithAPIKey(agent.ApiKey),
	)

	var tools []anthropic.ToolUnionParam

	for _, tool := range request.Tools {
		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
		}

		tools = append(
			tools,
			anthropic.ToolUnionParam{OfTool: &toolParam},
		)
	}

	messages, err := buildAnthropicMessages(request.Messages)
	if err != nil {
		return response, err
	}

	result, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     request.Model,
		MaxTokens: anthropicMaxTokens,
		Messages:  messages,
		Tools:     tools,
	})
	if err != nil {
		return response, err
	}

	for _, block := range result.Content {
		switch variant := block.AsAny().(type) {

		case anthropic.TextBlock:
			response.Content += variant.Text

		case anthropic.ToolUseBlock:

			args := make(map[string]any)

			if err := json.Unmarshal(variant.Input, &args); err != nil {
				return response, err
			}

			response.ToolCalls = append(
				response.ToolCalls,
				ai_model.ToolCall{
					ID:        variant.ID,
					Name:      variant.Name,
					Arguments: args,
				},
			)
		}
	}

	return response, nil
}


func buildAnthropicMessages(
	messages []ai_model.Message,
) ([]anthropic.MessageParam, error) {

	var result []anthropic.MessageParam
	var pendingToolResults []anthropic.ContentBlockParamUnion

	flushToolResults := func() {
		if len(pendingToolResults) > 0 {
			result = append(
				result,
				anthropic.NewUserMessage(pendingToolResults...),
			)
			pendingToolResults = nil
		}
	}

	for _, message := range messages {

		switch message.Role {

		case ai_model.RoleUser:
			flushToolResults()

			if message.Content != "" {
				result = append(
					result,
					anthropic.NewUserMessage(
						anthropic.NewTextBlock(message.Content),
					),
				)
			}

		case ai_model.RoleAssistant:
			flushToolResults()

			var blocks []anthropic.ContentBlockParamUnion

			if message.Content != "" {
				blocks = append(
					blocks,
					anthropic.NewTextBlock(message.Content),
				)
			}

			if message.ToolCalls != nil {
				for _, call := range *message.ToolCalls {
					blocks = append(
						blocks,
						anthropic.NewToolUseBlock(
							call.ID,
							call.Arguments,
							call.Name,
						),
					)
				}
			}

			if len(blocks) > 0 {
				result = append(
					result,
					anthropic.NewAssistantMessage(blocks...),
				)
			}

		case ai_model.RoleTool:
			if message.ToolResults != nil {
				for _, toolResult := range *message.ToolResults {

					content, err := json.Marshal(toolResult.Result)
					if err != nil {
						return nil, err
					}

					pendingToolResults = append(
						pendingToolResults,
						anthropic.NewToolResultBlock(
							toolResult.ToolCallID,
							string(content),
							false,
						),
					)
				}
			}
		}
	}

	flushToolResults()

	return result, nil
}
