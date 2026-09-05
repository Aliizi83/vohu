package chat

import (
	"fmt"
	"os"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/ai_model/models"
)

// buildLLM resolves a conversation's Provider into a concrete client,
// reading API keys from the same environment variables cmd/vohu's newLLM
// does. Server-side keys only for now — per-user BYO-key is real future
// work, out of scope here.
func buildLLM(provider string) (ai_model.LLM, error) {
	switch provider {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("GEMINI_API_KEY is not set")
		}
		return models.NewGeminiAgent(apiKey), nil

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY is not set")
		}
		// Optional: only needed for an identity-linked key tied to an
		// organization with more than one workspace.
		workspaceID := os.Getenv("ANTHROPIC_WORKSPACE_ID")
		return models.NewAnthropicAgent(apiKey, workspaceID), nil

	case "openai":
		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("OPENAI_API_KEY is not set")
		}
		// Empty baseURL means OpenAI itself; set OPENAI_BASE_URL for any
		// other OpenAI-compatible provider (DeepSeek, Groq, a local
		// Ollama server, ...).
		baseURL := os.Getenv("OPENAI_BASE_URL")
		return models.NewOpenAIAgent(apiKey, baseURL), nil

	default:
		return nil, fmt.Errorf("unknown provider: %q", provider)
	}
}
