package shared

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

// ResultCode mirrors sample-golang-project's api/helpers/error_codes.go
// taxonomy (HTTP-status-derived numeric codes), adapted to Vohu's actual
// error set.
type ResultCode int

const (
	ResultSuccess         ResultCode = 0
	ResultValidationError ResultCode = 40001
	ResultAuthError       ResultCode = 40101
	ResultForbiddenError  ResultCode = 40301
	ResultNotFoundError   ResultCode = 40401
	ResultConflictError   ResultCode = 40901
	ResultInternalError   ResultCode = 50001
)

type ValidationError struct {
	Property string `json:"property"`
	Tag      string `json:"tag"`
	Value    string `json:"value,omitempty"`
	Message  string `json:"message"`
}

// BaseResponse is the single envelope every handler in the platform
// responds with — success or error — adapted from
// sample-golang-project's api/helpers/BaseHttpResponse.
type BaseResponse struct {
	Result           any               `json:"result,omitempty"`
	Success          bool              `json:"success"`
	ResultCode       ResultCode        `json:"resultCode"`
	ValidationErrors []ValidationError `json:"validationErrors,omitempty"`
	Error            string            `json:"error,omitempty"`
}

func RespondSuccess(c *gin.Context, status int, result any) {
	c.JSON(status, BaseResponse{
		Result:     result,
		Success:    true,
		ResultCode: ResultSuccess,
	})
}

// AbortWithError is RespondError's counterpart for middleware, which must
// stop the chain (AbortWithStatusJSON) rather than merely write the
// response (JSON).
func AbortWithError(c *gin.Context, status int, code ResultCode, err error) {
	c.AbortWithStatusJSON(status, BaseResponse{
		Success:    false,
		ResultCode: code,
		Error:      err.Error(),
	})
}

func RespondError(c *gin.Context, status int, code ResultCode, err error) {
	c.JSON(status, BaseResponse{
		Success:    false,
		ResultCode: code,
		Error:      err.Error(),
	})
}

// RespondValidationError handles both real validator.ValidationErrors
// (binding tag failures) and plain body-parse errors — the latter just
// won't have a ValidationErrors list, only the raw Error message.
func RespondValidationError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, BaseResponse{
		Success:          false,
		ResultCode:       ResultValidationError,
		ValidationErrors: extractValidationErrors(err),
		Error:            err.Error(),
	})
}

func extractValidationErrors(err error) []ValidationError {
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		return nil
	}

	result := make([]ValidationError, 0, len(ve))
	for _, fe := range ve {
		result = append(result, ValidationError{
			Property: fe.Field(),
			Tag:      fe.Tag(),
			Value:    fe.Param(),
			Message:  fe.Error(),
		})
	}

	return result
}
