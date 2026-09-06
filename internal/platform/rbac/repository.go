package rbac

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	CreateRole(ctx context.Context, r *Role) error
	FindRoleByID(ctx context.Context, id uint) (*Role, error)
	FindRoleByName(ctx context.Context, name string) (*Role, error)
	UpdateRole(ctx context.Context, r *Role) error
	DeleteRole(ctx context.Context, id uint) error
	ListRoles(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Role, int64, error)

	UserRoleExists(ctx context.Context, userID, roleID uint) (bool, error)
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error
	// GetRoleIDsForUser backs the resolver — both for the caller's own
	// roles (step 3 in HasAccessLevel) and, when checking access to a
	// "user" resource, for the target user's roles (the cascade in step
	// 4).
	GetRoleIDsForUser(ctx context.Context, userID uint) ([]uint, error)

	// UpsertResourceAccess finds an existing row for the exact
	// (granteeType, granteeID, resourceType, resourceID) tuple and
	// updates its Level/Effect, or creates a new one — a repeat grant
	// updates rather than duplicates.
	UpsertResourceAccess(ctx context.Context, granteeType GranteeType, granteeID uint, resourceType string, resourceID uint, level AccessLevel, effect ResourceEffect) error
	// FindResourceAccess looks up the exact tuple only — the resolver is
	// the one that tries the real ID then falls back to
	// shared.WildcardResourceID, not this method.
	FindResourceAccess(ctx context.Context, granteeType GranteeType, granteeID uint, resourceType string, resourceID uint) (*ResourceAccess, error)
	// FindResourceAccessByID looks up a grant row by its own ID (not the
	// resource it refers to) — used to learn a grant's ResourceType/
	// ResourceID before revoking it, so that can be checked against the
	// resolver in turn.
	FindResourceAccessByID(ctx context.Context, id uint) (*ResourceAccess, error)
	ListResourceAccess(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]ResourceAccess, int64, error)
	// ListResourceAccessForGrantee is hand-written rather than going
	// through shared.ApplyDynamicFilter — that helper's OpEquals builds an
	// ILIKE comparison (it exists for user-supplied filter UIs on text
	// columns), which breaks against GranteeID's real numeric column in
	// Postgres. Same reasoning as conversation.ListConversationsByUser.
	ListResourceAccessForGrantee(ctx context.Context, granteeType GranteeType, granteeID uint) ([]ResourceAccess, error)
	DeleteResourceAccess(ctx context.Context, id uint) error
}

// gormRepository holds one generic repository per entity it manages for
// plain CRUD, plus hand-written methods for everything beyond that: name
// lookups, the user<->role join table, and the resource-access resolver's
// exact-tuple lookups.
type gormRepository struct {
	db             *gorm.DB
	roles          *shared.GenericRepository[Role]
	resourceAccess *shared.GenericRepository[ResourceAccess]
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		db:             db,
		roles:          shared.NewGenericRepository[Role](db),
		resourceAccess: shared.NewGenericRepository[ResourceAccess](db),
	}
}

func (r *gormRepository) CreateRole(ctx context.Context, role *Role) error {
	return r.roles.Create(ctx, role)
}

func (r *gormRepository) FindRoleByID(ctx context.Context, id uint) (*Role, error) {
	return r.roles.FindByID(ctx, id)
}

func (r *gormRepository) UpdateRole(ctx context.Context, role *Role) error {
	return r.roles.Update(ctx, role)
}

func (r *gormRepository) DeleteRole(ctx context.Context, id uint) error {
	return r.roles.Delete(ctx, id)
}

func (r *gormRepository) ListRoles(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Role, int64, error) {
	return r.roles.List(ctx, filter, page)
}

func (r *gormRepository) FindRoleByName(ctx context.Context, name string) (*Role, error) {
	var role Role

	err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func (r *gormRepository) UserRoleExists(ctx context.Context, userID, roleID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&UserRole{}).
		Where("user_id = ? AND role_id = ?", userID, roleID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) AssignRoleToUser(ctx context.Context, userID, roleID uint) error {
	return r.db.WithContext(ctx).Create(&UserRole{
		UserID: userID,
		RoleID: roleID,
	}).Error
}

func (r *gormRepository) GetRoleIDsForUser(ctx context.Context, userID uint) ([]uint, error) {
	var roleIDs []uint
	err := r.db.WithContext(ctx).
		Model(&UserRole{}).
		Where("user_id = ?", userID).
		Pluck("role_id", &roleIDs).Error
	return roleIDs, err
}

func (r *gormRepository) UpsertResourceAccess(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
	level AccessLevel,
	effect ResourceEffect,
) error {
	existing, err := r.FindResourceAccess(ctx, granteeType, granteeID, resourceType, resourceID)
	if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return err
	}

	if existing != nil {
		existing.Level = level
		existing.Effect = effect
		return r.db.WithContext(ctx).Save(existing).Error
	}

	return r.db.WithContext(ctx).Create(&ResourceAccess{
		GranteeType:  granteeType,
		GranteeID:    granteeID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Level:        level,
		Effect:       effect,
	}).Error
}

func (r *gormRepository) FindResourceAccess(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
) (*ResourceAccess, error) {
	var access ResourceAccess

	err := r.db.WithContext(ctx).
		Where(
			"grantee_type = ? AND grantee_id = ? AND resource_type = ? AND resource_id = ?",
			granteeType, granteeID, resourceType, resourceID,
		).
		First(&access).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &access, nil
}

func (r *gormRepository) ListResourceAccess(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]ResourceAccess, int64, error) {
	return r.resourceAccess.List(ctx, filter, page)
}

func (r *gormRepository) FindResourceAccessByID(ctx context.Context, id uint) (*ResourceAccess, error) {
	return r.resourceAccess.FindByID(ctx, id)
}

func (r *gormRepository) ListResourceAccessForGrantee(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
) ([]ResourceAccess, error) {
	var access []ResourceAccess
	err := r.db.WithContext(ctx).
		Where("grantee_type = ? AND grantee_id = ?", granteeType, granteeID).
		Find(&access).Error
	return access, err
}

func (r *gormRepository) DeleteResourceAccess(ctx context.Context, id uint) error {
	return r.resourceAccess.Delete(ctx, id)
}
