package sshconn

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. Get/Update/Delete are
// gated per-:id (shared.RequireAccessLevelOnParam already tries the exact
// row then falls back to a caller's wildcard/role grant, closing the gap
// this used to have — Update/Delete were flat-permission-gated only,
// meaning anyone holding "ssh:update"/"ssh:delete" could touch any
// connection regardless of resource grants); List can't be expressed that
// way (no single :id, and visibility is per-row) so it's the one
// hand-written handler — see Service.ListForCaller.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	connections := v1.Group("/ssh-connections", authMiddleware)
	{
		connections.POST("", shared.RequireAccessLevelWildcard(hasAccessLevel, ResourceTypeSSHConnection, "write"), handler.Create)
		connections.GET("", handler.List)
		connections.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeSSHConnection, "read"), handler.Get)
		connections.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeSSHConnection, "write"), handler.Update)
		connections.DELETE("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeSSHConnection, "manage"), handler.Delete)
	}
}
