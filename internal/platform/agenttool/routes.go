package agenttool

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. No POST/DELETE — the
// catalog has no user-created rows (see Tool's doc comment). List can't
// be gated per-:id the way Get/Update are (no single :id, visibility is
// per-row and includes every public tool regardless of any grant), so
// it's the one hand-written handler — see Service.ListForCaller.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	toolsGroup := v1.Group("/agent-tools", authMiddleware)
	{
		toolsGroup.GET("", handler.List)
		toolsGroup.GET("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeTool, "read"), handler.Get)
		toolsGroup.PUT("/:id", shared.RequireAccessLevelOnParam(hasAccessLevel, ResourceTypeTool, "manage"), handler.Update)
	}
}
