package providerkey

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires two groups: /provider-keys/me (self-service, any
// authenticated user manages their own) and /provider-keys (the
// global/admin default, gated by Policy.CanManageGlobal).
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc, policy *Policy) {
	mine := v1.Group("/provider-keys/me", authMiddleware)
	{
		mine.POST("", handler.SetMine)
		mine.GET("", handler.ListMine)
		mine.DELETE("/:provider", handler.DeleteMine)
	}

	global := v1.Group("/provider-keys", authMiddleware, shared.RequirePolicy(policy.CanManageGlobal))
	{
		global.POST("", handler.SetGlobal)
		global.GET("", handler.ListGlobal)
		global.DELETE("/:provider", handler.DeleteGlobal)
	}
}
