package user

import "github.com/gin-gonic/gin"

// RegisterRoutes wires this module's HTTP routes. authMiddleware and
// requirePermission are injected by the composition root (cmd/server) so
// this module never has to import the auth or rbac modules itself.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	requirePermission func(key string) gin.HandlerFunc,
) {
	users := v1.Group("/users", authMiddleware)
	{
		users.POST("", requirePermission("user:create"), handler.Create)
		users.GET("/:id", requirePermission("user:read"), handler.Get)
	}
}
