package httpserver

import (
	"fmt"

	"github.com/Aliizi83/vohu/config"
	"github.com/Aliizi83/vohu/pkg/logging"
	"github.com/gin-gonic/gin"
)

// NewEngine builds a Gin engine with the platform's global middleware
// attached, and returns the /api/v1 group every module registers its
// routes onto. This package knows nothing about user/rbac/auth — the
// composition root (cmd/server) wires those in.
func NewEngine(logger logging.Logger) (*gin.Engine, *gin.RouterGroup) {
	engine := gin.New()
	engine.Use(Recovery(logger), RequestLogger(logger))

	v1 := engine.Group("/api/v1")

	return engine, v1
}

func Run(engine *gin.Engine, cfg *config.Config) error {
	return engine.Run(fmt.Sprintf(":%s", cfg.Server.InternalPort))
}
