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

	messages, err := buildAnthropicMessages(request.Messages)
	if err != nil {
		return response, err
	}

	result, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     request.Model,
		MaxTokens: anthropicMaxTokens,
		Messages:  messages,
		Tools:     buildAnthropicTools(request.Tools),
	})
	if err != nil {
		return response, err
	}

	if err := accumulateAnthropicResponse(&response, result.Content); err != nil {
		return response, err
	}

	return response, nil
}

func (agent *AnthropicAgent) StreamChat(
	ctx context.Context,
	request ai_model.ChatRequest,
	onChunk func(text string),
) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client := anthropic.NewClient(
		option.WithAPIKey(agent.ApiKey),
	)

	messages, err := buildAnthropicMessages(request.Messages)
	if err != nil {
		return response, err
	}

	stream := client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     request.Model,
		MaxTokens: anthropicMaxTokens,
		Messages:  messages,
		Tools:     buildAnthropicTools(request.Tools),
	})

	message := anthropic.Message{}

	for stream.Next() {

		event := stream.Current()

		if err := message.Accumulate(event); err != nil {
			return response, err
		}

		if delta, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent); ok {
			if delta.Delta.Text != "" && onChunk != nil {
				onChunk(delta.Delta.Text)
			}
		}
	}

	if err := stream.Err(); err != nil {
		return response, err
	}

	if err := accumulateAnthropicResponse(&response, message.Content); err != nil {
		return response, err
	}

	return response, nil
}

func buildAnthropicTools(definitions []ai_model.ToolDefinition) []anthropic.ToolUnionParam {

	var tools []anthropic.ToolUnionParam

	for _, tool := range definitions {

		toolParam := anthropic.ToolParam{
			Name:        tool.Name,
			Description: anthropic.String(tool.Description),
		}

		if properties := jsonSchemaProperties(tool.Parameters); properties != nil {
			toolParam.InputSchema = anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   tool.Parameters.Required,
			}
		}

		tools = append(
			tools,
			anthropic.ToolUnionParam{OfTool: &toolParam},
		)
	}

	return tools
}

// accumulateAnthropicResponse folds a message's content blocks (from either
// the non-streaming response or the fully-accumulated streamed message)
// into response.
func accumulateAnthropicResponse(
	response *ai_model.ChatResponse,
	content []anthropic.ContentBlockUnion,
) error {

	for _, block := range content {
		switch variant := block.AsAny().(type) {

		case anthropic.TextBlock:
			response.Content += variant.Text

		case anthropic.ToolUseBlock:

			args := make(map[string]any)

			if err := json.Unmarshal(variant.Input, &args); err != nil {
				return err
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

	return nil
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
