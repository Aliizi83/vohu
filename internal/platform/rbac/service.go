package rbac

import (
	"context"
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// Service is what other modules (auth, user, and eventually the agent
// tooling itself) depend on — never Repository directly.
type Service interface {
	EnsureRole(ctx context.Context, name string) (*Role, error)
	EnsurePermission(ctx context.Context, key string) (*Permission, error)
	ListAllPermissions(ctx context.Context) ([]Permission, error)

	GrantPermissionToRole(ctx context.Context, roleID, permissionID uint) error
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error

	HasPermission(ctx context.Context, userID uint, key string) (bool, error)

	// RequirePermission builds Gin middleware that 403s unless the
	// authenticated user (read from shared.UserIDContextKey, set by the
	// auth module's middleware) holds the given permission key.
	RequirePermission(key string) gin.HandlerFunc
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

// EnsureRole/EnsurePermission are idempotent create-if-missing helpers,
// used by both admin-management handlers and startup seeding.

func (s *service) EnsureRole(ctx context.Context, name string) (*Role, error) {
	role, err := s.repo.FindRoleByName(ctx, name)
	if err == nil {
		return role, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	role = &Role{Name: name}
	if err := s.repo.CreateRole(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *service) EnsurePermission(ctx context.Context, key string) (*Permission, error) {
	permission, err := s.repo.FindPermissionByKey(ctx, key)
	if err == nil {
		return permission, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}

	permission = &Permission{Key: key}
	if err := s.repo.CreatePermission(ctx, permission); err != nil {
		return nil, err
	}

	return permission, nil
}

func (s *service) ListAllPermissions(ctx context.Context) ([]Permission, error) {
	return s.repo.ListAllPermissions(ctx)
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

func (s *service) RequirePermission(key string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := shared.GetUserID(c)
		if !ok {
			shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
			return
		}

		allowed, err := s.HasPermission(c.Request.Context(), userID, key)
		if err != nil {
			shared.AbortWithError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
			return
		}
		if !allowed {
			shared.AbortWithError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
			return
		}

		c.Next()
	}
}
