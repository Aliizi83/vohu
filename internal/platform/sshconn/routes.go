package sshconn

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. GET routes are open to
// any authenticated user (not gated by policy.CanRead) because visibility
// is no longer all-or-nothing: Handler.List/Get call the service's
// ...ForCaller methods, which show a flat-ssh:read holder everything and
// everyone else only the connections they hold resource-level access to.
// Mutating routes stay exactly as before — flat-permission-gated only.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	policy *Policy,
) {
	connections := v1.Group("/ssh-connections", authMiddleware)
	{
		connections.POST("", shared.RequirePolicy(policy.CanCreate), handler.Create)
		connections.GET("", handler.List)
		connections.GET("/:id", handler.Get)
		connections.PUT("/:id", shared.RequirePolicy(policy.CanUpdate), handler.Update)
		connections.DELETE("/:id", shared.RequirePolicy(policy.CanDelete), handler.Delete)
	}
}
