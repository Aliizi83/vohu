package auth

import "github.com/gin-gonic/gin"

// RegisterRoutes wires this module's HTTP routes. Both are public — no
// auth middleware — since they're the entry point that produces tokens.
func RegisterRoutes(v1 *gin.RouterGroup, handler *Handler) {
	group := v1.Group("/auth")
	{
		group.POST("/login", handler.Login)
		group.POST("/refresh", handler.Refresh)
	}
}
