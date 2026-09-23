package shared

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AccessLevelCheck func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error)

const WildcardResourceID uint = 0

func RequireAccessLevelOnParam(check AccessLevelCheck, resourceType string, level string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := RequireUserID(c)
		if !ok {
			return
		}

		resourceID, ok := RequireIDParam(c)
		if !ok {
			return
		}

		allowed, err := check(c.Request.Context(), userID, resourceType, resourceID, level)
		if err != nil {
			AbortWithError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
			return
		}
		if !allowed {
			AbortWithError(c, http.StatusNotFound, ResultNotFoundError, errors.New("record not found"))
			return
		}

		c.Next()
	}
}

func RequireCustomCheckOnParam(check func(ctx context.Context, userID uint, resourceID uint) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := RequireUserID(c)
		if !ok {
			return
		}

		resourceID, ok := RequireIDParam(c)
		if !ok {
			return
		}

		if err := check(c.Request.Context(), userID, resourceID); err != nil {
			if errors.Is(err, ErrNotFound) {
				AbortWithError(c, http.StatusNotFound, ResultNotFoundError, err)
				return
			}
			AbortWithError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
			return
		}

		c.Next()
	}
}

func RequireAccessLevelWildcard(check AccessLevelCheck, resourceType string, level string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, ok := RequireUserID(c)
		if !ok {
			return
		}

		allowed, err := check(c.Request.Context(), userID, resourceType, WildcardResourceID, level)
		if err != nil {
			AbortWithError(c, http.StatusInternalServerError, ResultInternalError, errors.New("internal error"))
			return
		}
		if !allowed {
			AbortWithError(c, http.StatusForbidden, ResultForbiddenError, errors.New("forbidden"))
			return
		}

		c.Next()
	}
}
