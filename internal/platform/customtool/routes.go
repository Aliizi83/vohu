package customtool

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	tools := v1.Group("/custom-tools", authMiddleware)
	{
		tools.POST("", shared.RequireAccessLevelWildcard(hasAccessLevel, ResourceTypeCustomTool, "write"), handler.Create)
		tools.POST("/check-source", shared.RequireAccessLevelWildcard(hasAccessLevel, ResourceTypeCustomTool, "write"), handler.CheckSource)
		tools.GET("", handler.List)
		tools.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "read"), handler.Get)
		tools.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "write"), handler.Update)
		tools.DELETE("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "manage"), handler.Delete)
		tools.POST("/:id/versions", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "manage"), handler.CreateVersion)
		tools.GET("/:id/versions", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "read"), handler.ListVersions)
	}
}
