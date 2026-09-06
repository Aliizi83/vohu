package rbac

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. Every route is gated
// through Service.HasAccessLevel — rbac doesn't need shared.AccessLevelCheck
// injected from outside the way every other module does (that indirection
// exists so a module never has to import rbac just to check one
// resource's grant); rbac already owns the resolver, so it builds its own
// adapter locally to hand to shared.RequireAccessLevelOnParam/Wildcard,
// the same middleware every other module's routes use.
//
// GrantResourceAccess/RevokeResourceAccess aren't gated by that
// middleware at all — see their handler doc comments for why (the
// resource being granted/revoked lives in the request body or has to be
// looked up first, not read straight off a URL param).
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	service Service,
) {
	hasAccessLevel := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return service.HasAccessLevel(ctx, userID, resourceType, resourceID, AccessLevel(level))
	}

	roles := v1.Group("/roles", authMiddleware)
	{
		roles.POST("", shared.RequireAccessLevelWildcard(hasAccessLevel, "role", "write"), handler.CreateRole)
		roles.GET("", shared.RequireAccessLevelWildcard(hasAccessLevel, "role", "read"), handler.ListRoles)
		roles.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "role", "read"), handler.GetRole)
		roles.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "role", "write"), handler.UpdateRole)
		roles.DELETE("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "role", "manage"), handler.DeleteRole)
	}

	// :id here is the target USER's ID — assigning them a role is a
	// management action on that user resource.
	v1.POST(
		"/users/:id/roles", authMiddleware,
		shared.RequireAccessLevelOnParam(hasAccessLevel, "user", "manage"),
		handler.AssignRoleToUser,
	)

	resourceAccess := v1.Group("/resource-access", authMiddleware)
	{
		resourceAccess.POST("", handler.GrantResourceAccess)
		resourceAccess.GET("", shared.RequireAccessLevelWildcard(hasAccessLevel, "resource_access", "read"), handler.ListResourceAccess)
		resourceAccess.DELETE("/:id", handler.RevokeResourceAccess)
	}

	v1.GET("/me/access", authMiddleware, handler.GetMyAccess)
}
