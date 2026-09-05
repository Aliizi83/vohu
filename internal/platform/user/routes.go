package user

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. authMiddleware and
// policy are injected by the composition root (cmd/server) so this module
// never has to import the auth or rbac modules itself.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	policy *Policy,
) {
	users := v1.Group("/users", authMiddleware)
	{
		users.POST("", shared.RequirePolicy(policy.CanCreate), handler.Create)
		users.GET("", shared.RequirePolicy(policy.CanRead), handler.List)
		users.GET("/:id", shared.RequirePolicy(policy.CanRead), handler.Get)
		users.PUT("/:id", shared.RequirePolicy(policy.CanUpdate), handler.Update)
		users.DELETE("/:id", shared.RequirePolicy(policy.CanDelete), handler.Delete)
	}
}
