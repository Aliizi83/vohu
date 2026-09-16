package customtool

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	CreateTool(ctx context.Context, tool *Tool) error
	FindToolByID(ctx context.Context, id uint) (*Tool, error)
	UpdateTool(ctx context.Context, tool *Tool) error
	// DeleteTool removes a tool's versions first, then the tool itself —
	// there's no DB foreign key between them to cascade on (ToolVersion.ToolID
	// is a plain column, not a gorm foreign key, same decoupling-by-convention
	// every module already follows for cross-entity references), same
	// pattern as conversation.Repository.DeleteConversation.
	DeleteTool(ctx context.Context, id uint) error
	ListTools(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Tool, int64, error)

	CreateVersion(ctx context.Context, version *ToolVersion) error
	ListVersionsForTool(ctx context.Context, toolID uint, page shared.Pagination) ([]ToolVersion, int64, error)
	// LatestVersionForTool is whatever ToolVersion for this tool has the
	// highest ID — "latest" means "most recently created," not a
	// semver-aware comparison of Version strings.
	LatestVersionForTool(ctx context.Context, toolID uint) (*ToolVersion, error)
}

type gormRepository struct {
	db       *gorm.DB
	tools    *shared.GenericRepository[Tool]
	versions *shared.GenericRepository[ToolVersion]
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		db:       db,
		tools:    shared.NewGenericRepository[Tool](db),
		versions: shared.NewGenericRepository[ToolVersion](db),
	}
}

func (r *gormRepository) CreateTool(ctx context.Context, tool *Tool) error {
	return r.tools.Create(ctx, tool)
}

func (r *gormRepository) FindToolByID(ctx context.Context, id uint) (*Tool, error) {
	return r.tools.FindByID(ctx, id)
}

func (r *gormRepository) UpdateTool(ctx context.Context, tool *Tool) error {
	return r.tools.Update(ctx, tool)
}

func (r *gormRepository) DeleteTool(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Where("tool_id = ?", id).Delete(&ToolVersion{}).Error; err != nil {
		return err
	}
	return r.tools.Delete(ctx, id)
}

func (r *gormRepository) ListTools(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Tool, int64, error) {
	return r.tools.List(ctx, filter, page)
}

func (r *gormRepository) CreateVersion(ctx context.Context, version *ToolVersion) error {
	return r.versions.Create(ctx, version)
}

func (r *gormRepository) ListVersionsForTool(
	ctx context.Context,
	toolID uint,
	page shared.Pagination,
) ([]ToolVersion, int64, error) {
	var versions []ToolVersion
	var total int64

	query := r.db.WithContext(ctx).Where("tool_id = ?", toolID)

	if err := query.Session(&gorm.Session{}).Model(&ToolVersion{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("id desc").Offset(page.Offset()).Limit(page.Limit()).Find(&versions).Error
	if err != nil {
		return nil, 0, err
	}

	return versions, total, nil
}

func (r *gormRepository) LatestVersionForTool(ctx context.Context, toolID uint) (*ToolVersion, error) {
	var version ToolVersion

	err := r.db.WithContext(ctx).Where("tool_id = ?", toolID).Order("id desc").First(&version).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &version, nil
}
