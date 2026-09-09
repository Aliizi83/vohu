package custommodel

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires two groups, the same shape providerkey's routes
// use: /custom-models/me (self-service, any authenticated user manages
// their own presets, ownership enforced in the repository) and
// /custom-models (the global/admin presets, gated as a whole by a
// wildcard "manage" check on resourceType "custom_model" — same reasoning
// as provider keys, these aren't addressed by a caller-facing numeric ID
// scheme worth gating per-row).
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc, hasAccessLevel shared.AccessLevelCheck) {
	mine := v1.Group("/custom-models/me", authMiddleware)
	{
		mine.POST("", handler.CreateMine)
		mine.GET("", handler.ListMine)
		mine.DELETE("/:id", handler.DeleteMine)
	}

	// The new-chat picker's read path — every authenticated user needs
	// this regardless of whether they can manage global presets, unlike
	// every other route in this file.
	v1.GET("/custom-models/available", authMiddleware, handler.ListAccessible)

	manageGlobal := shared.RequireAccessLevelWildcard(hasAccessLevel, "custom_model", "manage")
	global := v1.Group("/custom-models", authMiddleware, manageGlobal)
	{
		global.POST("", handler.CreateGlobal)
		global.GET("", handler.ListGlobal)
		global.DELETE("/:id", handler.DeleteGlobal)
	}
}
