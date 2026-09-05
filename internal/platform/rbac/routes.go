package rbac

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires this module's HTTP routes. Every route here is
// admin-management surface, so all of it requires policy.CanManage
// ("rbac:manage", granted to the default admin role at seed time).
func RegisterRoutes(
	v1 *gin.RouterGroup,
	handler *Handler,
	authMiddleware gin.HandlerFunc,
	policy *Policy,
) {
	manage := shared.RequirePolicy(policy.CanManage)

	roles := v1.Group("/roles", authMiddleware)
	{
		roles.POST("", manage, handler.CreateRole)
		roles.POST("/:id/permissions", manage, handler.GrantPermissionToRole)
	}

	permissions := v1.Group("/permissions", authMiddleware)
	{
		permissions.POST("", manage, handler.CreatePermission)
		permissions.GET("", manage, handler.ListPermissions)
	}

	v1.POST("/users/:id/roles", authMiddleware, manage, handler.AssignRoleToUser)
}
