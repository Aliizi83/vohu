package commandrule

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// Service is a plain trusted CRUD layer — same as sshconn.Service.List
// being "the raw unfiltered query," it has no RBAC awareness of its own.
// Whether a caller may reach a given connection's rules is decided by
// Handler (a hand-written check, same reasoning as
// rbac.Handler.GrantResourceAccess — the connection a request concerns
// lives in the body or has to be looked up first, not read straight off a
// URL param, so it can't be expressed as route middleware) for HTTP
// callers, and independently by chat.SSHTool for its own execute-time
// read (already cleared the caller at whatever level SSHTool itself
// requires before ever calling ListForConnection — see chat/ssh_tool.go).
type Service interface {
	Create(ctx context.Context, req CreateRuleRequest) (*Rule, error)
	GetByID(ctx context.Context, id uint) (*Rule, error)
	Update(ctx context.Context, id uint, req UpdateRuleRequest) (*Rule, error)
	Delete(ctx context.Context, id uint) error
	ListForConnection(ctx context.Context, sshConnectionID uint, page shared.Pagination) ([]Rule, int64, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, req CreateRuleRequest) (*Rule, error) {
	allowed := true
	if req.Allowed != nil {
		allowed = *req.Allowed
	}

	rule := &Rule{
		SSHConnectionID: req.SSHConnectionID,
		Program:         req.Program,
		ArgsPrefixes:    ArgsPrefixes(req.ArgsPrefixes),
		Allowed:         allowed,
	}
	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*Rule, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Update(ctx context.Context, id uint, req UpdateRuleRequest) (*Rule, error) {
	rule, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Program != "" {
		rule.Program = req.Program
	}
	if req.ArgsPrefixes != nil {
		rule.ArgsPrefixes = ArgsPrefixes(req.ArgsPrefixes)
	}
	if req.Allowed != nil {
		rule.Allowed = *req.Allowed
	}

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, err
	}

	return rule, nil
}

func (s *service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) ListForConnection(
	ctx context.Context,
	sshConnectionID uint,
	page shared.Pagination,
) ([]Rule, int64, error) {
	return s.repo.ListForConnection(ctx, sshConnectionID, page)
}
