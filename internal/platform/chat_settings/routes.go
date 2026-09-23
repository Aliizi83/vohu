package chat_settings

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes under chat's own
// /conversations group — chat_settings has no resource of its own to
// list or create, only a per-conversation settings sub-route. Both routes
// are gated by shared.RequireCustomCheckOnParam(canAccess, ...) the same
// way sshconn's Get/Update are gated by RequireAccessLevelOnParam, so the
// handler methods stay the plain generic CRUD handlers.
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc, canAccess ConversationAccessCheck) {
	requireAccess := shared.RequireCustomCheckOnParam(canAccess)

	conversations := v1.Group("/conversations", authMiddleware)
	{
		conversations.GET("/:id/settings", requireAccess, handler.Get)
		conversations.PUT("/:id/settings", requireAccess, handler.Update)
	}
}
