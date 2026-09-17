package terminal

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes adds two routes under the existing /ssh-connections
// prefix (registered separately from sshconn.RegisterRoutes, but the
// same base path — gin allows this as long as no method+path collides).
// terminal-ticket is a normal authenticated request; terminal-ws
// deliberately isn't (see Handler.ServeWS's doc comment) — a WebSocket
// handshake can't carry the Bearer header the rest of the API relies on.
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	hasAccessLevel shared.AccessLevelCheck,
) {
	connections := v1.Group("/ssh-connections")
	{
		connections.POST("/:id/terminal-ticket",
			authMiddleware,
			shared.RequireAccessLevelOnParam(hasAccessLevel, sshconn.ResourceTypeSSHConnection, "write"),
			handler.IssueTicket,
		)
		connections.GET("/:id/terminal-ws", handler.ServeWS)
	}
}
