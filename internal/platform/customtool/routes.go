package customtool

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. Create needs a wildcard
// "write" check (there's no existing row yet to check access on — same
// pattern as sshconn.RegisterRoutes' POST); every other route concerns one
// specific tool identified by :id, so shared.RequireAccessLevelOnParam
// handles it directly, including the two /versions routes (:id is still
// the tool's own ID there, versions have no independent access story of
// their own).
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	tools := v1.Group("/custom-tools", authMiddleware)
	{
		tools.POST("", shared.RequireAccessLevelWildcard(hasAccessLevel, ResourceTypeCustomTool, "write"), handler.Create)
		tools.GET("", handler.List)
		tools.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "read"), handler.Get)
		tools.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "write"), handler.Update)
		tools.DELETE("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "manage"), handler.Delete)
		tools.POST("/:id/versions", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "manage"), handler.CreateVersion)
		tools.GET("/:id/versions", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeCustomTool, "read"), handler.ListVersions)
	}
}
