package models

import (
	"context"
	"encoding/json"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

const openAIMaxTokens = 16000

type OpenAIAgent struct {
	ApiKey string
	BaseURL string
}

func NewOpenAIAgent(apiKey string, baseURL string) *OpenAIAgent {
	return &OpenAIAgent{
		ApiKey:  apiKey,
		BaseURL: baseURL,
	}
}

func (agent *OpenAIAgent) newClient() openai.Client {

	options := []option.RequestOption{
		option.WithAPIKey(agent.ApiKey),
	}

	if agent.BaseURL != "" {
		options = append(options, option.WithBaseURL(agent.BaseURL))
	}

	return openai.NewClient(options...)
}

func (agent *OpenAIAgent) Chat(
	ctx context.Context,
	request ai_model.ChatRequest,
) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client := agent.newClient()

	messages, err := buildOpenAIMessages(request.Messages)
	if err != nil {
		return response, err
	}

	result, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:     request.Model,
		MaxTokens: openai.Int(openAIMaxTokens),
		Messages:  messages,
		Tools:     buildOpenAITools(request.Tools),
	})
	if err != nil {
		return response, err
	}

	if len(result.Choices) == 0 {
		return response, nil
	}

	if err := accumulateOpenAIMessage(&response, result.Choices[0].Message); err != nil {
		return response, err
	}

	return response, nil
}

func (agent *OpenAIAgent) StreamChat(
	ctx context.Context,
	request ai_model.ChatRequest,
	onChunk func(text string),
) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client := agent.newClient()

	messages, err := buildOpenAIMessages(request.Messages)
	if err != nil {
		return response, err
	}

	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:     request.Model,
		MaxTokens: openai.Int(openAIMaxTokens),
		Messages:  messages,
		Tools:     buildOpenAITools(request.Tools),
	})

	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {

		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) == 0 {
			continue
		}

		if delta := chunk.Choices[0].Delta.Content; delta != "" && onChunk != nil {
			onChunk(delta)
		}
	}

	if err := stream.Err(); err != nil {
		return response, err
	}

	if len(acc.Choices) == 0 {
		return response, nil
	}

	if err := accumulateOpenAIMessage(&response, acc.Choices[0].Message); err != nil {
		return response, err
	}

	return response, nil
}

func buildOpenAITools(definitions []ai_model.ToolDefinition) []openai.ChatCompletionToolParam {

	var tools []openai.ChatCompletionToolParam

	for _, tool := range definitions {

		function := shared.FunctionDefinitionParam{
			Name:        tool.Name,
			Description: openai.String(tool.Description),
		}

		if properties := jsonSchemaProperties(tool.Parameters); properties != nil {

			schema := shared.FunctionParameters{
				"type":       "object",
				"properties": properties,
			}

			if len(tool.Parameters.Required) > 0 {
				schema["required"] = tool.Parameters.Required
			}

			function.Parameters = schema
		}

		tools = append(tools, openai.ChatCompletionToolParam{Function: function})
	}

	return tools
}

func accumulateOpenAIMessage(
	response *ai_model.ChatResponse,
	message openai.ChatCompletionMessage,
) error {

	response.Content += message.Content

	for _, call := range message.ToolCalls {

		args := make(map[string]any)

		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			return err
		}

		response.ToolCalls = append(
			response.ToolCalls,
			ai_model.ToolCall{
				ID:        call.ID,
				Name:      call.Function.Name,
				Arguments: args,
			},
		)
	}

	return nil
}

func buildOpenAIMessages(
	messages []ai_model.Message,
) ([]openai.ChatCompletionMessageParamUnion, error) {

	var result []openai.ChatCompletionMessageParamUnion

	for _, message := range messages {

		switch message.Role {

		case ai_model.RoleUser:
			if message.Content != "" {
				result = append(result, openai.UserMessage(message.Content))
			}

		case ai_model.RoleAssistant:

			if message.ToolCalls != nil && len(*message.ToolCalls) > 0 {

				toolCalls := make(
					[]openai.ChatCompletionMessageToolCallParam,
					0,
					len(*message.ToolCalls),
				)

				for _, call := range *message.ToolCalls {

					arguments, err := json.Marshal(call.Arguments)
					if err != nil {
						return nil, err
					}

					toolCalls = append(toolCalls, openai.ChatCompletionMessageToolCallParam{
						ID: call.ID,
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Name,
							Arguments: string(arguments),
						},
					})
				}

				assistant := openai.ChatCompletionAssistantMessageParam{
					ToolCalls: toolCalls,
				}

				if message.Content != "" {
					assistant.Content.OfString = param.NewOpt(message.Content)
				}

				result = append(
					result,
					openai.ChatCompletionMessageParamUnion{OfAssistant: &assistant},
				)

			} else if message.Content != "" {
				result = append(result, openai.AssistantMessage(message.Content))
			}

		case ai_model.RoleTool:
			if message.ToolResults != nil {
				for _, toolResult := range *message.ToolResults {

					content, err := json.Marshal(toolResult.Result)
					if err != nil {
						return nil, err
					}

					result = append(
						result,
						openai.ToolMessage(string(content), toolResult.ToolCallID),
					)
				}
			}
		}
	}

	return result, nil
}
