package providerkey

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/crypto"
)

// ResolvedCredential is what chat's buildLLM actually needs to construct
// a provider client — this package knows nothing about ai_model.LLM or
// the models.New*Agent constructors, keeping the same "who imports what"
// discipline every module in this codebase follows.
type ResolvedCredential struct {
	APIKey      string
	BaseURL     string
	WorkspaceID string
}

type Service interface {
	// SetGlobal and the Global* methods are gated by Policy.CanManage at
	// the route level — see routes.go/policy.go.
	SetGlobal(ctx context.Context, req SetKeyRequest) error
	ListGlobal(ctx context.Context) ([]Response, error)
	DeleteGlobal(ctx context.Context, provider Provider) error

	// SetMine/ListMine/DeleteMine are self-service — no policy beyond
	// being authenticated, same as conversation ownership.
	SetMine(ctx context.Context, userID uint, req SetKeyRequest) error
	ListMine(ctx context.Context, userID uint) ([]Response, error)
	DeleteMine(ctx context.Context, userID uint, provider Provider) error

	// Resolve is the actual lookup chat.buildLLM calls: the caller's own
	// key first, then the global/admin-set key, then the legacy
	// provider-specific environment variables (GEMINI_API_KEY, ...) as a
	// last resort so a fresh dev checkout still works with zero DB setup.
	Resolve(ctx context.Context, userID uint, provider Provider) (ResolvedCredential, error)
}

type service struct {
	repo Repository
	box  *crypto.Box
}

func NewService(repo Repository, box *crypto.Box) Service {
	return &service{repo: repo, box: box}
}

func (s *service) SetGlobal(ctx context.Context, req SetKeyRequest) error {
	encrypted, err := s.box.Encrypt(req.APIKey)
	if err != nil {
		return err
	}
	return s.repo.UpsertGlobal(ctx, req.Provider, encrypted, req.BaseURL, req.WorkspaceID)
}

func (s *service) ListGlobal(ctx context.Context) ([]Response, error) {
	keys, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return nil, err
	}
	return toResponses(keys), nil
}

func (s *service) DeleteGlobal(ctx context.Context, provider Provider) error {
	return s.repo.DeleteGlobal(ctx, provider)
}

func (s *service) SetMine(ctx context.Context, userID uint, req SetKeyRequest) error {
	encrypted, err := s.box.Encrypt(req.APIKey)
	if err != nil {
		return err
	}
	return s.repo.UpsertForUser(ctx, userID, req.Provider, encrypted, req.BaseURL, req.WorkspaceID)
}

func (s *service) ListMine(ctx context.Context, userID uint) ([]Response, error) {
	keys, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toResponses(keys), nil
}

func (s *service) DeleteMine(ctx context.Context, userID uint, provider Provider) error {
	return s.repo.DeleteForUser(ctx, userID, provider)
}

func (s *service) Resolve(ctx context.Context, userID uint, provider Provider) (ResolvedCredential, error) {
	if key, err := s.repo.FindForUser(ctx, userID, provider); err == nil {
		return s.decrypt(key)
	} else if !errors.Is(err, shared.ErrNotFound) {
		return ResolvedCredential{}, err
	}

	if key, err := s.repo.FindGlobal(ctx, provider); err == nil {
		return s.decrypt(key)
	} else if !errors.Is(err, shared.ErrNotFound) {
		return ResolvedCredential{}, err
	}

	if cred, ok := resolveFromEnv(provider); ok {
		return cred, nil
	}

	return ResolvedCredential{}, fmt.Errorf(
		"no API key configured for provider %q — set your own key, ask an admin to set a global one, or set %s_API_KEY",
		provider, strings.ToUpper(string(provider)),
	)
}

func (s *service) decrypt(key *Key) (ResolvedCredential, error) {
	apiKey, err := s.box.Decrypt(key.EncryptedAPIKey)
	if err != nil {
		return ResolvedCredential{}, err
	}
	return ResolvedCredential{APIKey: apiKey, BaseURL: key.BaseURL, WorkspaceID: key.WorkspaceID}, nil
}

// resolveFromEnv mirrors cmd/vohu's newLLM and chat's original
// buildLLM — the same env vars, preserved as a fallback for a zero-setup
// dev checkout once the DB becomes the primary path.
func resolveFromEnv(provider Provider) (ResolvedCredential, bool) {
	switch provider {
	case ProviderGemini:
		if v := os.Getenv("GEMINI_API_KEY"); v != "" {
			return ResolvedCredential{APIKey: v}, true
		}
	case ProviderAnthropic:
		if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" {
			return ResolvedCredential{APIKey: v, WorkspaceID: os.Getenv("ANTHROPIC_WORKSPACE_ID")}, true
		}
	case ProviderOpenAI:
		if v := os.Getenv("OPENAI_API_KEY"); v != "" {
			return ResolvedCredential{APIKey: v, BaseURL: os.Getenv("OPENAI_BASE_URL")}, true
		}
	}
	return ResolvedCredential{}, false
}

func toResponses(keys []Key) []Response {
	responses := make([]Response, 0, len(keys))
	for _, key := range keys {
		responses = append(responses, toResponse(key))
	}
	return responses
}
