package shared

import "github.com/gin-gonic/gin"

// UserIDContextKey is the gin.Context key the auth module's Authentication
// middleware sets, and the rbac module's RequirePermission middleware
// reads. Living here means neither module has to import the other just to
// agree on a string.
const UserIDContextKey = "user_id"

func GetUserID(c *gin.Context) (uint, bool) {
	val, exists := c.Get(UserIDContextKey)
	if !exists {
		return 0, false
	}

	id, ok := val.(uint)
	return id, ok
}
