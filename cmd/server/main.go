package main

import (
	"github.com/Aliizi83/vohu/config"
	"github.com/Aliizi83/vohu/internal/platform/auth"
	"github.com/Aliizi83/vohu/internal/platform/httpserver"
	"github.com/Aliizi83/vohu/internal/platform/migrations"
	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"github.com/Aliizi83/vohu/pkg/db"
	"github.com/Aliizi83/vohu/pkg/logging"
)

func main() {
	cfg := config.GetConfig()
	logger := logging.NewLogger(cfg)

	if err := db.InitDB(cfg); err != nil {
		logger.Fatal(err, logging.Postgres, logging.Startup, err.Error(), nil)
	}
	defer db.CloseDB()

	if err := migrations.UpP_1(db.GetDB(), logger); err != nil {
		logger.Fatal(err, logging.Postgres, logging.Migration, err.Error(), nil)
	}

	userRepo := user.NewRepository(db.GetDB())
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	rbacRepo := rbac.NewRepository(db.GetDB())
	rbacService := rbac.NewService(rbacRepo)
	rbacHandler := rbac.NewHandler(rbacService)

	authService := auth.NewService(cfg, userService)
	authHandler := auth.NewHandler(authService)

	engine, v1 := httpserver.NewEngine(logger)

	authMiddleware := authService.Authentication()

	user.RegisterRoutes(v1, userHandler, authMiddleware, rbacService.RequirePermission)
	rbac.RegisterRoutes(v1, rbacHandler, authMiddleware, rbacService.RequirePermission)
	auth.RegisterRoutes(v1, authHandler)

	logger.Info(logging.General, logging.Startup, "starting vohu server", nil)

	if err := httpserver.Run(engine, cfg); err != nil {
		logger.Fatal(err, logging.General, logging.Startup, err.Error(), nil)
	}
}
