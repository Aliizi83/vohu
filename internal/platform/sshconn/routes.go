package sshconn

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	policy *Policy,
) {
	connections := v1.Group("/ssh-connections", authMiddleware)
	{
		connections.POST("", shared.RequirePolicy(policy.CanCreate), handler.Create)
		connections.GET("", shared.RequirePolicy(policy.CanRead), handler.List)
		connections.GET("/:id", shared.RequirePolicy(policy.CanRead), handler.Get)
		connections.PUT("/:id", shared.RequirePolicy(policy.CanUpdate), handler.Update)
		connections.DELETE("/:id", shared.RequirePolicy(policy.CanDelete), handler.Delete)
	}
}
