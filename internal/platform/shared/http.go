package shared

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// ParseIDParam reads the ":id" URL param as a uint. Shared by every
// module's handlers so the same parsing rule and error stay in one place.
func ParseIDParam(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}
