package commandrule

import "github.com/gin-gonic/gin"

// RegisterRoutes wires this module's HTTP routes. None of them go through
// shared.RequireAccessLevelOnParam/Wildcard — every one needs to learn
// which SSH connection is in play (from the body, a query param, or a
// rule lookup) before it knows what to check, so each handler does its
// own hand-written access check instead (see handler.go).
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
) {
	rules := v1.Group("/command-rules", authMiddleware)
	{
		rules.POST("", handler.Create)
		rules.GET("", handler.List)
		rules.PUT("/:id", handler.Update)
		rules.DELETE("/:id", handler.Delete)
	}
}
