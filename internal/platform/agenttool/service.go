package agenttool

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// Service is what chat.Handler depends on to build a conversation's tool
// registry — never Repository directly. There is no Create/Delete: the
// catalog's rows are the fixed set seeders.seedAgentTools writes at
// startup, matched by name to a concrete Go implementation in
// internal/platform/chat (see chat/registry.go) — see Tool's doc comment.
type Service interface {
	GetByID(ctx context.Context, id uint) (*Tool, error)
	// Update only ever changes Visibility, see UpdateToolRequest.
	Update(ctx context.Context, id uint, req UpdateToolRequest) (*Tool, error)
	// List is the raw, unfiltered query — used internally by
	// ListForCaller's wildcard-access fast path.
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Tool, int64, error)

	// ListForCaller is what chat.Handler calls to build a conversation's
	// tool registry: every VisibilityPublic row, plus any
	// VisibilityPrivate row the caller holds at least Read-level resource
	// access to. A caller holding wildcard "read" sees every row
	// unfiltered (unchanged admin behavior, same as every other module's
	// ListForCaller).
	ListForCaller(ctx context.Context, userID uint, filter shared.DynamicFilter, page shared.Pagination) ([]Tool, int64, error)
}

type service struct {
	repo           Repository
	hasAccessLevel shared.AccessLevelCheck
}

func NewService(repo Repository, hasAccessLevel shared.AccessLevelCheck) Service {
	return &service{repo: repo, hasAccessLevel: hasAccessLevel}
}

func (s *service) GetByID(ctx context.Context, id uint) (*Tool, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Update(ctx context.Context, id uint, req UpdateToolRequest) (*Tool, error) {
	t, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	t.Visibility = req.Visibility

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, err
	}

	return t, nil
}

func (s *service) List(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Tool, int64, error) {
	return s.repo.List(ctx, filter, page)
}

func (s *service) ListForCaller(
	ctx context.Context,
	userID uint,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Tool, int64, error) {
	canSeeAll, err := s.hasAccessLevel(ctx, userID, ResourceTypeTool, shared.WildcardResourceID, "read")
	if err != nil {
		return nil, 0, err
	}
	if canSeeAll {
		return s.repo.List(ctx, filter, page)
	}

	// shared.FilterAndPaginate doesn't fit here — it applies
	// hasAccessLevel uniformly to every candidate, with no way to skip
	// the check entirely for a public row, so the filtering loop (and its
	// pagination-over-the-filtered-slice math) is duplicated by hand
	// instead.
	candidates, _, err := s.repo.List(ctx, filter, shared.Pagination{PageNumber: 1, PageSize: 1000})
	if err != nil {
		return nil, 0, err
	}

	visible := make([]Tool, 0, len(candidates))
	for _, t := range candidates {
		if t.Visibility == VisibilityPublic {
			visible = append(visible, t)
			continue
		}
		ok, err := s.hasAccessLevel(ctx, userID, ResourceTypeTool, t.ID, "read")
		if err != nil {
			return nil, 0, err
		}
		if ok {
			visible = append(visible, t)
		}
	}

	total := int64(len(visible))
	start := page.Offset()
	if start > len(visible) {
		start = len(visible)
	}
	end := start + page.Limit()
	if end > len(visible) {
		end = len(visible)
	}

	return visible[start:end], total, nil
}
