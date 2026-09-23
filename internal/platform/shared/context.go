package shared

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

const UserIDContextKey = "user_id"

var (
	ErrUnauthenticated = errors.New("unauthenticated")
	ErrInvalidID       = errors.New("invalid id")
)

func GetUserID(c *gin.Context) (uint, bool) {
	val, exists := c.Get(UserIDContextKey)
	if !exists {
		return 0, false
	}

	id, ok := val.(uint)
	return id, ok
}

func RequireUserID(c *gin.Context) (uint, bool) {
	userID, ok := GetUserID(c)
	if !ok {
		AbortWithError(c, http.StatusUnauthorized, ResultAuthError, ErrUnauthenticated)
		return 0, false
	}
	return userID, true
}

// RequireIDParam uses AbortWithError rather than RespondError even though
// most of its callers are plain handlers (where the distinction doesn't
// matter — nothing runs after the last handler either way) — it's also
// called from inside RequireAccessLevelOnParam/RequireCustomCheckOnParam
// (policy.go), genuine middleware where failing to stop the chain lets
// the real handler run anyway and write its own second response.
func RequireIDParam(c *gin.Context) (uint, bool) {
	id, err := ParseIDParam(c)
	if err != nil {
		AbortWithError(c, http.StatusBadRequest, ResultValidationError, ErrInvalidID)
		return 0, false
	}
	return id, true
}
