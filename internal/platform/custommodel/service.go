package custommodel

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/crypto"
)

// ResolvedCredential is what chat.buildLLM needs to construct an OpenAI
// client for one specific preset — this package knows nothing about
// ai_model.LLM, keeping the same "who imports what" discipline every
// module in this codebase follows (mirrors providerkey.ResolvedCredential).
type ResolvedCredential struct {
	APIKey  string
	BaseURL string
}

type Service interface {
	// CreateMine/ListMine/DeleteMine are self-service — no policy beyond
	// being authenticated and owning the row, same as provider keys' Mine
	// methods.
	CreateMine(ctx context.Context, userID uint, req CreateCustomModelRequest) (Response, error)
	ListMine(ctx context.Context, userID uint) ([]Response, error)
	DeleteMine(ctx context.Context, userID uint, id uint) error

	// CreateGlobal/ListGlobal/DeleteGlobal are gated by a wildcard
	// "manage" check on "custom_model" at the route level — see routes.go.
	CreateGlobal(ctx context.Context, req CreateCustomModelRequest) (Response, error)
	ListGlobal(ctx context.Context) ([]Response, error)
	DeleteGlobal(ctx context.Context, id uint) error

	// ListAccessible is deliberately NOT gated by "manage" — it's the
	// picker list for starting a new chat, which every authenticated user
	// needs to read regardless of whether they're allowed to create or
	// delete global presets. Mine + global combined, since both are
	// choices the caller can actually pick.
	ListAccessible(ctx context.Context, userID uint) ([]Response, error)

	// Resolve is the actual lookup chat.buildLLM calls once a conversation
	// names a specific preset — it 404s (shared.ErrNotFound) rather than
	// 403s if the preset belongs to someone else, same "don't confirm it
	// exists" reasoning conversation.Service.Get uses for a conversation
	// the caller can't see.
	Resolve(ctx context.Context, callerUserID uint, id uint) (ResolvedCredential, error)
}

type service struct {
	repo Repository
	box  *crypto.Box
}

func NewService(repo Repository, box *crypto.Box) Service {
	return &service{repo: repo, box: box}
}

func (s *service) CreateMine(ctx context.Context, userID uint, req CreateCustomModelRequest) (Response, error) {
	return s.create(ctx, &userID, req)
}

func (s *service) ListMine(ctx context.Context, userID uint) ([]Response, error) {
	models, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toResponses(models), nil
}

func (s *service) DeleteMine(ctx context.Context, userID uint, id uint) error {
	return s.repo.DeleteForUser(ctx, userID, id)
}

func (s *service) CreateGlobal(ctx context.Context, req CreateCustomModelRequest) (Response, error) {
	return s.create(ctx, nil, req)
}

func (s *service) ListGlobal(ctx context.Context) ([]Response, error) {
	models, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return nil, err
	}
	return toResponses(models), nil
}

func (s *service) DeleteGlobal(ctx context.Context, id uint) error {
	return s.repo.DeleteGlobal(ctx, id)
}

func (s *service) ListAccessible(ctx context.Context, userID uint) ([]Response, error) {
	mine, err := s.repo.ListForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	global, err := s.repo.ListGlobal(ctx)
	if err != nil {
		return nil, err
	}
	return toResponses(append(mine, global...)), nil
}

func (s *service) create(ctx context.Context, userID *uint, req CreateCustomModelRequest) (Response, error) {
	encrypted, err := s.box.Encrypt(req.APIKey)
	if err != nil {
		return Response{}, err
	}

	m := &CustomModel{
		UserID:          userID,
		Name:            req.Name,
		BaseURL:         req.BaseURL,
		ModelName:       req.ModelName,
		EncryptedAPIKey: encrypted,
	}
	if err := s.repo.Create(ctx, m); err != nil {
		return Response{}, err
	}

	return toResponse(*m), nil
}

func (s *service) Resolve(ctx context.Context, callerUserID uint, id uint) (ResolvedCredential, error) {
	m, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ResolvedCredential{}, err
	}

	if m.UserID != nil && *m.UserID != callerUserID {
		return ResolvedCredential{}, shared.ErrNotFound
	}

	apiKey, err := s.box.Decrypt(m.EncryptedAPIKey)
	if err != nil {
		return ResolvedCredential{}, err
	}

	return ResolvedCredential{APIKey: apiKey, BaseURL: m.BaseURL}, nil
}
