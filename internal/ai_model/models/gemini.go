package models

import (
	"context"
	"maps"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"google.golang.org/genai"
)

type ModelName string

const (
	GeminiFlash      ModelName = "gemini-3.7-flash"
	GeminiFlashLight35 ModelName = "gemini-3.5-flash-lite"
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

		// Normal text message
		if message.Content != "" {

			var role genai.Role

			if message.Role == ai_model.RoleAssistant {
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

		if message.ToolCalls != nil && len(*message.ToolCalls) > 0 {

			parts := make([]*genai.Part, 0, len(*message.ToolCalls))

			for _, call := range *message.ToolCalls {

				var thoughtSignature []byte

				if call.Metadata != nil {
					if googleMetadata, ok := call.Metadata["google"].(map[string]any); ok {
						if signature, ok := googleMetadata["thought_signature"].([]byte); ok {
							thoughtSignature = signature
						}
					}
				}

				parts = append(parts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						ID:   call.ID,
						Name: call.Name,
						Args: call.Arguments,
					},
					ThoughtSignature: thoughtSignature,
				})
			}

			contents = append(
				contents,
				&genai.Content{
					Role:  genai.RoleModel,
					Parts: parts,
				},
			)
		}

		if message.ToolResults != nil && len(*message.ToolResults) > 0 {

			parts := make([]*genai.Part, 0, len(*message.ToolResults))

			for _, result := range *message.ToolResults {

				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						ID:   result.ToolCallID,
						Name: result.Name,
						Response: map[string]any{
							"result": result.Result,
						},
					},
				})
			}

			contents = append(
				contents,
				&genai.Content{
					Role:  genai.RoleUser,
					Parts: parts,
				},
			)
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

				toolCall := ai_model.ToolCall{
					ID:        part.FunctionCall.ID,
					Name:      part.FunctionCall.Name,
					Arguments: args,
					Metadata: map[string]any{
						"google": map[string]any{
							"thought_signature": part.ThoughtSignature,
						},
					},
				}

				response.ToolCalls = append(
					response.ToolCalls,
					toolCall,
				)
			}
		}
	}

	return response, nil
}
