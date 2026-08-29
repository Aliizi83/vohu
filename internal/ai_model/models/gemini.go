package models

import (
	"context"
	"maps"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/google/uuid"
	"google.golang.org/genai"
)

type ModelName string

const (
	GeminiFlash ModelName = "gemini-3.7-flash"
)

type GeminiAgent struct {
	ApiKey string
}

func NewGeminiAgent(apiKey string) *GeminiAgent {
	return &GeminiAgent{
		ApiKey: apiKey,
	}
}

func (agent *GeminiAgent) Chat(
	ctx context.Context,
	request ai_model.ChatRequest,
) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  agent.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return response, err
	}

	var functionDeclarations []*genai.FunctionDeclaration

	for _, tool := range request.Tools {
		functionDeclarations = append(
			functionDeclarations,
			&genai.FunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
			},
		)
	}

	var geminiTools []*genai.Tool

	if len(functionDeclarations) > 0 {
		geminiTools = []*genai.Tool{
			{
				FunctionDeclarations: functionDeclarations,
			},
		}
	}

	var contents []*genai.Content

	for _, message := range request.Messages {

		if message.Content != "" {

			var role genai.Role

			if message.Role == "assistant" {
				role = genai.RoleModel
			}

			contents = append(
				contents,
				genai.NewContentFromText(
					message.Content,
					role,
				),
			)
		}

		// Tool calls
		if message.ToolCalls != nil && len(*message.ToolCalls) > 0 {

			for _, call := range *message.ToolCalls {

				id := uuid.New().String()

				contents = append(
					contents,
					&genai.Content{
						Role: genai.RoleModel,
						Parts: []*genai.Part{
							{
								FunctionCall: &genai.FunctionCall{
									ID:   id,
									Name: call.Name,
									Args: call.Arguments,
								},
							},
						},
					},
				)
			}
		}

		// Tool results
		if message.ToolResults != nil && len(*message.ToolResults) > 0 {

			for _, result := range *message.ToolResults {

				contents = append(
					contents,
					&genai.Content{
						Role: genai.RoleUser,
						Parts: []*genai.Part{
							{
								FunctionResponse: &genai.FunctionResponse{
									Name: result.Name,
									Response: map[string]any{
										"result": result.Result,
									},
								},
							},
						},
					},
				)
			}
		}
	}

	config := &genai.GenerateContentConfig{
		Tools: geminiTools,
	}

	result, err := client.Models.GenerateContent(
		ctx,
		request.Model,
		contents,
		config,
	)

	if err != nil {
		return response, err
	}

	for _, candidate := range result.Candidates {

		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {

			if part.Text != "" {
				response.Content += part.Text
			}

			if part.FunctionCall != nil {

				args := make(map[string]any)

				maps.Copy(args, part.FunctionCall.Args)

				response.ToolCalls = append(
					response.ToolCalls,
					ai_model.ToolCall{
						Name:      part.FunctionCall.Name,
						Arguments: args,
					},
				)
			}
		}
	}

	return response, nil
}
