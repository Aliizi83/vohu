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

	CreatePermission(ctx context.Context, p *Permission) error
	FindPermissionByID(ctx context.Context, id uint) (*Permission, error)
	FindPermissionByKey(ctx context.Context, key string) (*Permission, error)
	UpdatePermission(ctx context.Context, p *Permission) error
	DeletePermission(ctx context.Context, id uint) error
	ListPermissions(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Permission, int64, error)

	RolePermissionExists(ctx context.Context, roleID, permissionID uint) (bool, error)
	AssignPermissionToRole(ctx context.Context, roleID, permissionID uint) error

	UserRoleExists(ctx context.Context, userID, roleID uint) (bool, error)
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error

	GetPermissionKeysForUser(ctx context.Context, userID uint) ([]string, error)

	UpsertResourcePermission(ctx context.Context, userID uint, resourceType string, resourceID uint, effect Effect) error
	FindResourcePermission(ctx context.Context, userID uint, resourceType string, resourceID uint) (*ResourcePermission, error)
}

// gormRepository holds one generic repository per entity it manages (named
// fields, not embedding — Role and Permission would both promote a
// same-named FindByID/etc, which is ambiguous through embedding) for plain
// CRUD, plus the hand-written methods for everything beyond that:
// name/key lookups, the role<->permission and user<->role join tables, and
// the permission-keys-for-a-user query.
type gormRepository struct {
	db          *gorm.DB
	roles       *shared.GenericRepository[Role]
	permissions *shared.GenericRepository[Permission]
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		db:          db,
		roles:       shared.NewGenericRepository[Role](db),
		permissions: shared.NewGenericRepository[Permission](db),
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

func (r *gormRepository) CreatePermission(ctx context.Context, permission *Permission) error {
	return r.permissions.Create(ctx, permission)
}

func (r *gormRepository) FindPermissionByID(ctx context.Context, id uint) (*Permission, error) {
	return r.permissions.FindByID(ctx, id)
}

func (r *gormRepository) UpdatePermission(ctx context.Context, permission *Permission) error {
	return r.permissions.Update(ctx, permission)
}

func (r *gormRepository) DeletePermission(ctx context.Context, id uint) error {
	return r.permissions.Delete(ctx, id)
}

func (r *gormRepository) ListPermissions(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Permission, int64, error) {
	return r.permissions.List(ctx, filter, page)
}

func (r *gormRepository) FindPermissionByKey(ctx context.Context, key string) (*Permission, error) {
	var permission Permission

	err := r.db.WithContext(ctx).Where("key = ?", key).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &permission, nil
}

func (r *gormRepository) RolePermissionExists(ctx context.Context, roleID, permissionID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&RolePermission{}).
		Where("role_id = ? AND permission_id = ?", roleID, permissionID).
		Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) AssignPermissionToRole(ctx context.Context, roleID, permissionID uint) error {
	return r.db.WithContext(ctx).Create(&RolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
	}).Error
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

func (r *gormRepository) UpsertResourcePermission(
	ctx context.Context,
	userID uint,
	resourceType string,
	resourceID uint,
	effect Effect,
) error {
	existing, err := r.FindResourcePermission(ctx, userID, resourceType, resourceID)
	if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return err
	}

	if existing != nil {
		existing.Effect = effect
		return r.db.WithContext(ctx).Save(existing).Error
	}

	return r.db.WithContext(ctx).Create(&ResourcePermission{
		UserID:       userID,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Effect:       effect,
	}).Error
}

func (r *gormRepository) FindResourcePermission(
	ctx context.Context,
	userID uint,
	resourceType string,
	resourceID uint,
) (*ResourcePermission, error) {
	var permission ResourcePermission

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND resource_type = ? AND resource_id = ?", userID, resourceType, resourceID).
		First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &permission, nil
}

func (r *gormRepository) GetPermissionKeysForUser(ctx context.Context, userID uint) ([]string, error) {
	var keys []string

	err := r.db.WithContext(ctx).
		Table("permissions").
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id AND role_permissions.deleted_at IS NULL").
		Joins("JOIN user_roles ON user_roles.role_id = role_permissions.role_id AND user_roles.deleted_at IS NULL").
		Where("user_roles.user_id = ? AND permissions.deleted_at IS NULL", userID).
		Distinct().
		Pluck("permissions.key", &keys).Error

	return keys, err
}
