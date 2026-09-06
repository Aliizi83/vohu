package user

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. hasAccessLevel is
// injected by the composition root (cmd/server) so this module never has
// to import rbac itself. Get/Update/Delete are simple per-:id checks
// (shared.RequireAccessLevelOnParam already tries the exact row then
// falls back to a caller's wildcard grant, so there's nothing more for
// this module's handlers to do); List can't be expressed that way (no
// single :id, and visibility is per-row) so it's the one hand-written
// handler — see Service.ListForCaller.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	users := v1.Group("/users", authMiddleware)
	{
		users.POST("", shared.RequireAccessLevelWildcard(hasAccessLevel, "user", "write"), handler.Create)
		users.GET("", handler.List)
		users.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "user", "read"), handler.Get)
		users.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "user", "write"), handler.Update)
		users.DELETE("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, "user", "manage"), handler.Delete)
	}
}
