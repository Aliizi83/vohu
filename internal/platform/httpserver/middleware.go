package httpserver

import (
	"net/http"
	"time"

	"github.com/Aliizi83/vohu/pkg/logging"
	"github.com/gin-gonic/gin"
)

// RequestLogger logs method/path/status/latency for every request. It
// deliberately does not log request/response bodies the way
// sample-golang-project's structuredLogger did — auth endpoints carry
// plaintext passwords and tokens, and logging those would be a real
// security issue, not a debugging convenience.
func RequestLogger(logger logging.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()

		c.Next()

		logger.Info(logging.RequestResponse, logging.Api, "", map[string]any{
			"method":     c.Request.Method,
			"path":       path,
			"status":     c.Writer.Status(),
			"latency_ms": time.Since(start).Milliseconds(),
			"client_ip":  c.ClientIP(),
		})
	}
}

func Recovery(logger logging.Logger) gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		if err, ok := recovered.(error); ok {
			logger.Error(err, logging.Internal, logging.Api, "panic recovered", nil)
		} else {
			logger.Error(nil, logging.Internal, logging.Api, "panic recovered", map[string]any{"recovered": recovered})
		}

		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	})
}
