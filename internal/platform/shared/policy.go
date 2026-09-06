package shared

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AccessLevelCheck is the shape of rbac.Service.HasAccessLevel — level is
// a plain string (rbac.AccessLevel's underlying type) so a module never
// has to import rbac just to check one resource's grant. This is the one
// authorization primitive every module's routes are built on now — there
// is no separate flat-permission-key mechanism anymore.
type AccessLevelCheck func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error)

// WildcardResourceID (0 — never a real GORM auto-increment ID) marks a
// grant that applies to every resource of a ResourceType rather than one
// specific row. It's how collection-level actions (create a new X, list
// all X) get checked — there's no existing row to point a grant at yet —
// and how the seeded admin role gets blanket access without a row per
// current-and-future object. Lives here rather than in rbac (which owns
// the rest of the resource-access schema) purely so this package's own
// middleware below can reference it without importing rbac.
const WildcardResourceID uint = 0

// RequireAccessLevelOnParam builds Gin middleware for routes that operate
// on one existing resource named by the ":id" URL param (GET one, PUT,
// DELETE, ...). A denial 404s rather than 403s — same reasoning as
// sshconn.Service.GetByIDForCaller: a resource the caller has no access to
// shouldn't even confirm its own existence to them.
func RequireAccessLevelOnParam(check AccessLevelCheck, resourceType string, level string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, ResultAuthError, errors.New("unauthenticated"))
			return
		}

		resourceID, err := ParseIDParam(c)
		if err != nil {
			AbortWithError(c, http.StatusBadRequest, ResultValidationError, errors.New("invalid id"))
			return
		}

		allowed, err := check(c.Request.Context(), userID, resourceType, resourceID, level)
		if err != nil {
			AbortWithError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
			return
		}
		if !allowed {
			AbortWithError(c, http.StatusNotFound, ResultNotFoundError, errors.New("record not found"))
			return
		}

		c.Next()
	}
}

// RequireAccessLevelWildcard builds Gin middleware for routes that don't
// operate on one existing resource (POST create, GET list) — checked
// against WildcardResourceID instead of a URL param. There's no specific
// object to hide the existence of here, so a denial 403s rather than 404s.
func RequireAccessLevelWildcard(check AccessLevelCheck, resourceType string, level string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			AbortWithError(c, http.StatusUnauthorized, ResultAuthError, errors.New("unauthenticated"))
			return
		}

		allowed, err := check(c.Request.Context(), userID, resourceType, WildcardResourceID, level)
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
