package providerkey

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires two groups: /provider-keys/me (self-service, any
// authenticated user manages their own) and /provider-keys (the
// global/admin default). The global group is gated as a whole — a
// wildcard "manage" check on resourceType "provider_key" — rather than
// per-:id like other modules' mutating routes, since a provider key isn't
// addressed by a numeric ID in these routes at all (DELETE takes the
// provider name, e.g. "gemini", not a row ID).
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc, hasAccessLevel shared.AccessLevelCheck) {
	mine := v1.Group("/provider-keys/me", authMiddleware)
	{
		mine.POST("", handler.SetMine)
		mine.GET("", handler.ListMine)
		mine.DELETE("/:provider", handler.DeleteMine)
	}

	manageGlobal := shared.RequireAccessLevelWildcard(hasAccessLevel, "provider_key", "manage")
	global := v1.Group("/provider-keys", authMiddleware, manageGlobal)
	{
		global.POST("", handler.SetGlobal)
		global.GET("", handler.ListGlobal)
		global.DELETE("/:provider", handler.DeleteGlobal)
	}
}
