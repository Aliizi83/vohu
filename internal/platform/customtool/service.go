package customtool

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

type GrantCreatorAccess func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string, effect string) error

// Service is what chat.Handler will depend on once execution is wired up
// in a later phase — this is storage only.
type Service interface {
	CreateTool(ctx context.Context, userID uint, req CreateToolRequest) (*Tool, error)
	GetToolByID(ctx context.Context, id uint) (*Tool, error)
	UpdateTool(ctx context.Context, id uint, req UpdateToolRequest) (*Tool, error)
	DeleteTool(ctx context.Context, id uint) error
	ListTools(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Tool, int64, error)
	ListToolsForCaller(ctx context.Context, userID uint, filter shared.DynamicFilter, page shared.Pagination) ([]Tool, int64, error)

	CreateVersion(ctx context.Context, userID uint, toolID uint, req CreateVersionRequest) (*ToolVersion, error)
	ListVersionsForTool(ctx context.Context, toolID uint, page shared.Pagination) ([]ToolVersion, int64, error)
	LatestVersionForTool(ctx context.Context, toolID uint) (*ToolVersion, error)
}

type service struct {
	repo           Repository
	grantAccess    GrantCreatorAccess
	hasAccessLevel shared.AccessLevelCheck
}

func NewService(repo Repository, grantAccess GrantCreatorAccess, hasAccessLevel shared.AccessLevelCheck) Service {
	return &service{repo: repo, grantAccess: grantAccess, hasAccessLevel: hasAccessLevel}
}

func (s *service) CreateTool(ctx context.Context, userID uint, req CreateToolRequest) (*Tool, error) {
	visibility := req.Visibility
	if visibility == "" {
		visibility = VisibilityPrivate
	}

	tool := &Tool{
		Name:            req.Name,
		Description:     req.Description,
		ParamsSchema:    req.ParamsSchema,
		Visibility:      visibility,
		CreatedByUserID: userID,
	}

	if err := s.repo.CreateTool(ctx, tool); err != nil {
		return nil, err
	}

	if err := s.grantAccess(ctx, userID, ResourceTypeCustomTool, tool.ID, "manage", "accepted"); err != nil {
		return nil, err
	}

	return tool, nil
}

func (s *service) GetToolByID(ctx context.Context, id uint) (*Tool, error) {
	return s.repo.FindToolByID(ctx, id)
}

func (s *service) UpdateTool(ctx context.Context, id uint, req UpdateToolRequest) (*Tool, error) {
	tool, err := s.repo.FindToolByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Description != "" {
		tool.Description = req.Description
	}
	if req.ParamsSchema != "" {
		tool.ParamsSchema = req.ParamsSchema
	}
	if req.Visibility != "" {
		tool.Visibility = req.Visibility
	}

	if err := s.repo.UpdateTool(ctx, tool); err != nil {
		return nil, err
	}

	return tool, nil
}

func (s *service) DeleteTool(ctx context.Context, id uint) error {
	return s.repo.DeleteTool(ctx, id)
}

func (s *service) ListTools(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Tool, int64, error) {
	return s.repo.ListTools(ctx, filter, page)
}

func (s *service) ListToolsForCaller(
	ctx context.Context,
	userID uint,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Tool, int64, error) {
	canSeeAll, err := s.hasAccessLevel(ctx, userID, ResourceTypeCustomTool, shared.WildcardResourceID, "read")
	if err != nil {
		return nil, 0, err
	}
	if canSeeAll {
		return s.repo.ListTools(ctx, filter, page)
	}

	candidates, _, err := s.repo.ListTools(ctx, filter, shared.Pagination{PageNumber: 1, PageSize: 1000})
	if err != nil {
		return nil, 0, err
	}

	visible := make([]Tool, 0, len(candidates))
	for _, t := range candidates {
		if t.Visibility == VisibilityPublic {
			visible = append(visible, t)
			continue
		}
		ok, err := s.hasAccessLevel(ctx, userID, ResourceTypeCustomTool, t.ID, "read")
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

func (s *service) CreateVersion(ctx context.Context, userID uint, toolID uint, req CreateVersionRequest) (*ToolVersion, error) {
	if _, err := s.repo.FindToolByID(ctx, toolID); err != nil {
		return nil, err
	}

	version := &ToolVersion{
		ToolID:          toolID,
		Version:         req.Version,
		SourceCode:      req.SourceCode,
		CreatedByUserID: userID,
	}

	if err := s.repo.CreateVersion(ctx, version); err != nil {
		return nil, err
	}

	return version, nil
}

func (s *service) ListVersionsForTool(
	ctx context.Context,
	toolID uint,
	page shared.Pagination,
) ([]ToolVersion, int64, error) {
	return s.repo.ListVersionsForTool(ctx, toolID, page)
}

func (s *service) LatestVersionForTool(ctx context.Context, toolID uint) (*ToolVersion, error) {
	return s.repo.LatestVersionForTool(ctx, toolID)
}
