package chat

import "github.com/gin-gonic/gin"

// RegisterRoutes wires this module's HTTP routes. There's no separate
// Policy here (unlike user/rbac/sshconn) — every route is scoped to the
// authenticated caller's own conversations by construction
// (conversation.Service.Get/LoadHistory already 404 a non-owner), so
// there's no separate "may I do this at all" permission to gate beyond
// being logged in.
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler, authMiddleware gin.HandlerFunc) {
	conversations := v1.Group("/conversations", authMiddleware)
	{
		conversations.POST("", handler.CreateConversation)
		conversations.GET("", handler.ListConversations)
		conversations.GET("/:id/messages", handler.GetMessages)
		conversations.POST("/:id/messages", handler.SendMessage)
	}
}
