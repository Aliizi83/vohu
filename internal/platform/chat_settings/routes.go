package chat_settings

import "github.com/gin-gonic/gin"

// RegisterRoutes wires this module's HTTP routes under chat's own
// /conversations group — chat_settings has no resource of its own to
// list or create, only a per-conversation settings sub-route.
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc) {
	conversations := v1.Group("/conversations", authMiddleware)
	{
		conversations.GET("/:id/settings", handler.Get)
		conversations.PUT("/:id/settings", handler.Update)
	}
}
