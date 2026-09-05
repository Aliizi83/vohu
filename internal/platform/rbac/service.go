package rbac

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var (
	ErrRoleExists       = errors.New("role already exists")
	ErrPermissionExists = errors.New("permission already exists")
)

// Service is what other modules (auth, user, and eventually the agent
// tooling itself) depend on — never Repository directly.
type Service interface {
	CreateRole(ctx context.Context, req CreateRoleRequest) (*Role, error)
	GetRole(ctx context.Context, id uint) (*Role, error)
	UpdateRole(ctx context.Context, id uint, req UpdateRoleRequest) (*Role, error)
	DeleteRole(ctx context.Context, id uint) error
	ListRoles(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Role, int64, error)

	CreatePermission(ctx context.Context, req CreatePermissionRequest) (*Permission, error)
	GetPermission(ctx context.Context, id uint) (*Permission, error)
	UpdatePermission(ctx context.Context, id uint, req UpdatePermissionRequest) (*Permission, error)
	DeletePermission(ctx context.Context, id uint) error
	ListPermissions(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Permission, int64, error)

	GrantPermissionToRole(ctx context.Context, roleID, permissionID uint) error
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error

	HasPermission(ctx context.Context, userID uint, key string) (bool, error)

	// GrantResourceAccess records an explicit Accepted/Forbidden decision
	// for one user on one (resourceType, resourceID) pair — an upsert, a
	// repeat grant just updates the effect.
	GrantResourceAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, effect Effect) error

	// CanAccessResource answers "can this user touch this specific row" —
	// default deny when no row exists at all, same safe-by-default posture
	// as command.CommandPolicy's accept mode.
	CanAccessResource(ctx context.Context, userID uint, resourceType string, resourceID uint) (bool, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateRole(ctx context.Context, req CreateRoleRequest) (*Role, error) {
	_, err := s.repo.FindRoleByName(ctx, req.Name)
	if err == nil {
		return nil, ErrRoleExists
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}

	role := &Role{Name: req.Name}
	if err := s.repo.CreateRole(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *service) GetRole(ctx context.Context, id uint) (*Role, error) {
	return s.repo.FindRoleByID(ctx, id)
}

func (s *service) UpdateRole(ctx context.Context, id uint, req UpdateRoleRequest) (*Role, error) {
	role, err := s.repo.FindRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		role.Name = req.Name
	}

	if err := s.repo.UpdateRole(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *service) DeleteRole(ctx context.Context, id uint) error {
	return s.repo.DeleteRole(ctx, id)
}

func (s *service) ListRoles(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Role, int64, error) {
	return s.repo.ListRoles(ctx, filter, page)
}

func (s *service) CreatePermission(ctx context.Context, req CreatePermissionRequest) (*Permission, error) {
	_, err := s.repo.FindPermissionByKey(ctx, req.Key)
	if err == nil {
		return nil, ErrPermissionExists
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}

	permission := &Permission{Key: req.Key}
	if err := s.repo.CreatePermission(ctx, permission); err != nil {
		return nil, err
	}

	return permission, nil
}

func (s *service) GetPermission(ctx context.Context, id uint) (*Permission, error) {
	return s.repo.FindPermissionByID(ctx, id)
}

func (s *service) UpdatePermission(ctx context.Context, id uint, req UpdatePermissionRequest) (*Permission, error) {
	permission, err := s.repo.FindPermissionByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Key != "" {
		permission.Key = req.Key
	}

	if err := s.repo.UpdatePermission(ctx, permission); err != nil {
		return nil, err
	}

	return permission, nil
}

func (s *service) DeletePermission(ctx context.Context, id uint) error {
	return s.repo.DeletePermission(ctx, id)
}

func (s *service) ListPermissions(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Permission, int64, error) {
	return s.repo.ListPermissions(ctx, filter, page)
}

func (s *service) GrantPermissionToRole(ctx context.Context, roleID, permissionID uint) error {
	exists, err := s.repo.RolePermissionExists(ctx, roleID, permissionID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return s.repo.AssignPermissionToRole(ctx, roleID, permissionID)
}

func (s *service) AssignRoleToUser(ctx context.Context, userID, roleID uint) error {
	exists, err := s.repo.UserRoleExists(ctx, userID, roleID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return s.repo.AssignRoleToUser(ctx, userID, roleID)
}

func (s *service) HasPermission(ctx context.Context, userID uint, key string) (bool, error) {
	keys, err := s.repo.GetPermissionKeysForUser(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, k := range keys {
		if k == key {
			return true, nil
		}
	}

	return false, nil
}

func (s *service) GrantResourceAccess(
	ctx context.Context,
	userID uint,
	resourceType string,
	resourceID uint,
	effect Effect,
) error {
	return s.repo.UpsertResourcePermission(ctx, userID, resourceType, resourceID, effect)
}

func (s *service) CanAccessResource(
	ctx context.Context,
	userID uint,
	resourceType string,
	resourceID uint,
) (bool, error) {
	permission, err := s.repo.FindResourcePermission(ctx, userID, resourceType, resourceID)
	if errors.Is(err, shared.ErrNotFound) {
		return false, nil // no row at all -> default deny
	}
	if err != nil {
		return false, err
	}

	return permission.Effect == EffectAccepted, nil
}
