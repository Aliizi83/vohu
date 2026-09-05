package chat

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/ai_model/models"
	"github.com/Aliizi83/vohu/internal/platform/providerkey"
)

// buildLLM resolves a conversation's Provider into a concrete client.
// Credential resolution itself (the caller's own key, falling back to the
// admin-set global key, falling back to the legacy environment variables)
// lives in providerkey.Service.Resolve — this function only turns the
// resolved secret into the right ai_model.LLM implementation.
func buildLLM(ctx context.Context, keys providerkey.Service, userID uint, provider string) (ai_model.LLM, error) {
	cred, err := keys.Resolve(ctx, userID, providerkey.Provider(provider))
	if err != nil {
		return nil, err
	}

	switch provider {
	case "gemini":
		return models.NewGeminiAgent(cred.APIKey), nil

	case "anthropic":
		// WorkspaceID is optional — only needed for an identity-linked key
		// tied to an organization with more than one workspace.
		return models.NewAnthropicAgent(cred.APIKey, cred.WorkspaceID), nil

	case "openai":
		// Empty BaseURL means OpenAI itself; a non-empty one points at any
		// other OpenAI-compatible provider (DeepSeek, Groq, a local Ollama
		// server, ...).
		return models.NewOpenAIAgent(cred.APIKey, cred.BaseURL), nil

	default:
		return nil, fmt.Errorf("unknown provider: %q", provider)
	}
}
