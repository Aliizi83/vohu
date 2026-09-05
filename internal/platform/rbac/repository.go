package rbac

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var ErrNotFound = errors.New("not found")

type Repository interface {
	CreateRole(ctx context.Context, r *Role) error
	FindRoleByName(ctx context.Context, name string) (*Role, error)

	CreatePermission(ctx context.Context, p *Permission) error
	FindPermissionByKey(ctx context.Context, key string) (*Permission, error)
	ListAllPermissions(ctx context.Context) ([]Permission, error)

	RolePermissionExists(ctx context.Context, roleID, permissionID uint) (bool, error)
	AssignPermissionToRole(ctx context.Context, roleID, permissionID uint) error

	UserRoleExists(ctx context.Context, userID, roleID uint) (bool, error)
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error

	GetPermissionKeysForUser(ctx context.Context, userID uint) ([]string, error)
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) CreateRole(ctx context.Context, role *Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

func (r *gormRepository) FindRoleByName(ctx context.Context, name string) (*Role, error) {
	var role Role

	err := r.db.WithContext(ctx).Where("name = ?", name).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &role, nil
}

func (r *gormRepository) CreatePermission(ctx context.Context, permission *Permission) error {
	return r.db.WithContext(ctx).Create(permission).Error
}

func (r *gormRepository) FindPermissionByKey(ctx context.Context, key string) (*Permission, error) {
	var permission Permission

	err := r.db.WithContext(ctx).Where("key = ?", key).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &permission, nil
}

func (r *gormRepository) ListAllPermissions(ctx context.Context) ([]Permission, error) {
	var permissions []Permission
	err := r.db.WithContext(ctx).Find(&permissions).Error
	return permissions, err
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
