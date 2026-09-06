package sshconn

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, conn *SSHConnection) error
	FindByID(ctx context.Context, id uint) (*SSHConnection, error)
	Update(ctx context.Context, conn *SSHConnection) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)

	// ListAccessibleToUser is List's counterpart for a caller who lacks
	// the flat "ssh:read" bypass — every caller-supplied filter still
	// applies, but a final pass narrows the result to rows this specific
	// user has at least Read-level resource access to. See its
	// implementation for why this is a raw SQL fragment rather than a
	// Go-level join.
	ListAccessibleToUser(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination, userID uint) ([]SSHConnection, int64, error)
}

// gormRepository embeds the generic repository for the plain-CRUD half
// (Create/FindByID/Update/Delete/List) and hand-writes
// ListAccessibleToUser, the one method that needs a query the generic
// repository can't express.
type gormRepository struct {
	*shared.GenericRepository[SSHConnection]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		GenericRepository: shared.NewGenericRepository[SSHConnection](db),
		db:                db,
	}
}

// ListAccessibleToUser references the rbac module's resource_permissions
// table by name only (no Go import of rbac — sshconn stays free of any
// dependency on rbac's package, same decoupling rule every module
// follows) in a raw EXISTS subquery. This is the one place in this
// module that couples to another module's schema rather than going
// through a function-value seam; a per-row visibility filter is exactly
// the kind of thing that has to run inside the database to stay
// query-level rather than fetch-everything-then-filter-in-Go, and an
// EXISTS subquery is the direct way to express that in SQL.
func (r *gormRepository) ListAccessibleToUser(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
	userID uint,
) ([]SSHConnection, int64, error) {
	var items []SSHConnection
	var total int64

	query, err := shared.ApplyDynamicFilter[SSHConnection](r.db.WithContext(ctx), filter)
	if err != nil {
		return nil, 0, err
	}

	query = query.Where(
		`EXISTS (
			SELECT 1 FROM resource_permissions rp
			WHERE rp.resource_type = ?
			  AND rp.resource_id = ssh_connections.id
			  AND rp.user_id = ?
			  AND rp.level IN ('read', 'write', 'manage')
			  AND rp.deleted_at IS NULL
		)`,
		ResourceTypeSSHConnection, userID,
	)

	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Model(&SSHConnection{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortQuery, err := shared.ApplySort[SSHConnection](query, filter.Sorts)
	if err != nil {
		return nil, 0, err
	}

	err = sortQuery.
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
