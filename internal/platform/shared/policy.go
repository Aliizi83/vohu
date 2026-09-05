package shared

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PermissionCheck is the shape of rbac.Service.HasPermission — the
// function every module's Policy is constructed with, so a module never
// has to import rbac just to check a permission key.
type PermissionCheck func(ctx context.Context, userID uint, key string) (bool, error)

// PolicyCheck is the shape every <module>.Policy method (CanCreate,
// CanRead, ...) has — takes the authenticated user's ID (read from
// UserIDContextKey), decides yes/no. A policy method can be a flat
// permission-key lookup (what every module currently does) or, later,
// something that needs the resource itself (ownership checks, ...) without
// changing this signature's callers.
type PolicyCheck func(ctx context.Context, userID uint) (bool, error)

// RequirePolicy builds Gin middleware that 403s unless check passes for
// the authenticated user. This is rbac.RequirePermission's counterpart at
// the routing layer, except routes call a named policy method
// (requirePolicy(policy.CanCreate)) instead of a raw permission-key string
// (requirePermission("user:create")) — the policy method is what actually
// decides which key(s), or whatever richer logic, to check.
func RequirePolicy(check PolicyCheck) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, ResultAuthError, errors.New("unauthenticated"))
			return
		}

		allowed, err := check(c.Request.Context(), userID)
		if err != nil {
			AbortWithError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
			return
		}
		if !allowed {
			AbortWithError(c, http.StatusForbidden, ResultForbiddenError, errors.New("forbidden"))
			return
		}

		c.Next()
	}
}
