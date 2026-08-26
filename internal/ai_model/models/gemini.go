package models

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"google.golang.org/genai"
)

type ModelName string

const (
	GeminiFlash ModelName = "gemini-3.7-flash"
)

type GeminiAgent struct {
	ApiKey    string
	ModelName ModelName
}

func NewGeminiAgent(apiKey string, modelName ModelName) *GeminiAgent {
	return &GeminiAgent{
		ApiKey:    apiKey,
		ModelName: modelName,
	}
}

func (agent *GeminiAgent) Chat(ctx context.Context, request ai_model.ChatRequest) (ai_model.ChatResponse, error) {

	var response ai_model.ChatResponse

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  agent.ApiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return response, fmt.Errorf("create Gemini client: %w", err)
	}

	contents := make([]*genai.Content, 0, len(request.Messages))

	for _, message := range request.Messages {
		var role genai.Role

		switch message.Role {
		case "user":
			role = genai.Role(genai.RoleUser)

		case "assistant":
			role = genai.Role(genai.RoleModel)

		default:
			return response, fmt.Errorf(
				"unsupported message role: %s",
				message.Role,
			)
		}

		contents = append(
			contents,
			genai.NewContentFromText(
				message.Content,
				role,
			),
		)
	}

	result, err := client.Models.GenerateContent(
		ctx,
		string(agent.ModelName),
		contents,
		nil,
	)
	if err != nil {
		return response, fmt.Errorf("generate content: %w", err)
	}

	response.Content = result.Text()

	return response, nil
}
