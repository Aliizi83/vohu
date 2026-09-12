package commandrule

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, rule *Rule) error
	FindByID(ctx context.Context, id uint) (*Rule, error)
	Update(ctx context.Context, rule *Rule) error
	Delete(ctx context.Context, id uint) error
	// ListForConnection is hand-written rather than going through
	// shared.ApplyDynamicFilter — same reasoning as
	// sshconn.gormRepository.List / rbac's ListResourceAccessForGrantee:
	// OpEquals builds an ILIKE comparison, which breaks against
	// SSHConnectionID's real numeric column in Postgres.
	ListForConnection(ctx context.Context, sshConnectionID uint, page shared.Pagination) ([]Rule, int64, error)
}

type gormRepository struct {
	db    *gorm.DB
	rules *shared.GenericRepository[Rule]
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db, rules: shared.NewGenericRepository[Rule](db)}
}

func (r *gormRepository) Create(ctx context.Context, rule *Rule) error {
	return r.rules.Create(ctx, rule)
}

func (r *gormRepository) FindByID(ctx context.Context, id uint) (*Rule, error) {
	return r.rules.FindByID(ctx, id)
}

func (r *gormRepository) Update(ctx context.Context, rule *Rule) error {
	return r.rules.Update(ctx, rule)
}

func (r *gormRepository) Delete(ctx context.Context, id uint) error {
	return r.rules.Delete(ctx, id)
}

func (r *gormRepository) ListForConnection(
	ctx context.Context,
	sshConnectionID uint,
	page shared.Pagination,
) ([]Rule, int64, error) {
	var rules []Rule
	var total int64

	query := r.db.WithContext(ctx).Where("ssh_connection_id = ?", sshConnectionID)

	if err := query.Session(&gorm.Session{}).Model(&Rule{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Offset(page.Offset()).Limit(page.Limit()).Find(&rules).Error
	if err != nil {
		return nil, 0, err
	}

	return rules, total, nil
}
