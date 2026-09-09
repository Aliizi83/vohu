package main

import (
	"context"

	"github.com/Aliizi83/vohu/config"
	"github.com/Aliizi83/vohu/internal/platform/auth"
	"github.com/Aliizi83/vohu/internal/platform/chat"
	"github.com/Aliizi83/vohu/internal/platform/conversation"
	"github.com/Aliizi83/vohu/internal/platform/custommodel"
	"github.com/Aliizi83/vohu/internal/platform/httpserver"
	"github.com/Aliizi83/vohu/internal/platform/migrations"
	"github.com/Aliizi83/vohu/internal/platform/providerkey"
	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"github.com/Aliizi83/vohu/pkg/crypto"
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

	rbacRepo := rbac.NewRepository(db.GetDB())
	rbacService := rbac.NewService(rbacRepo)
	rbacHandler := rbac.NewHandler(rbacService)

	// rbac.AccessLevel/ResourceEffect are named string types; every other
	// module's injected checks/hooks take plain strings so no module but
	// this one ever has to import rbac just to bridge the two.
	hasAccessLevel := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return rbacService.HasAccessLevel(ctx, userID, resourceType, resourceID, rbac.AccessLevel(level))
	}
	grantCreatorAccess := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string, effect string) error {
		return rbacService.GrantResourceAccess(
			ctx, rbac.GranteeUser, userID, resourceType, resourceID, rbac.AccessLevel(level), rbac.ResourceEffect(effect),
		)
	}

	userRepo := user.NewRepository(db.GetDB())
	userService := user.NewService(userRepo, hasAccessLevel)
	userHandler := user.NewHandler(userService)

	secretBox, err := crypto.NewBox(cfg.Secrets.EncryptionKey)
	if err != nil {
		logger.Fatal(err, logging.General, logging.Startup, err.Error(), nil)
	}

	sshconnRepo := sshconn.NewRepository(db.GetDB())
	sshconnService := sshconn.NewService(sshconnRepo, secretBox, grantCreatorAccess, hasAccessLevel)
	sshconnHandler := sshconn.NewHandler(sshconnService)

	authService := auth.NewService(cfg, userService)
	authHandler := auth.NewHandler(authService)

	conversationRepo := conversation.NewRepository(db.GetDB())
	conversationService := conversation.NewService(conversationRepo, hasAccessLevel)

	// Same allow-list the "command:*" keys used to describe under the old
	// flat-permission system — pwd/ls/whoami/git status/git log/docker
	// ps/docker logs — applied to every SSH connection for now (nothing
	// per-connection yet). Accept-mode (allow-list), unlike cmd/vohu's own
	// TESTING-ONLY deny-list, since this runs unattended against
	// user-supplied remote hosts rather than a developer's own machine.
	sshCommandPolicy := command.NewCommandPolicy(command.PolicyModeAccept, []command.Rule{
		{Program: "pwd", Allowed: true},
		{Program: "ls", Allowed: true},
		{Program: "whoami", Allowed: true},
		{Program: "git", ArgsPrefixes: [][]string{{"status"}, {"log"}}, Allowed: true},
		{Program: "docker", ArgsPrefixes: [][]string{{"ps"}, {"logs"}}, Allowed: true},
	})
	providerKeyRepo := providerkey.NewRepository(db.GetDB())
	providerKeyService := providerkey.NewService(providerKeyRepo, secretBox)
	providerKeyHandler := providerkey.NewHandler(providerKeyService)

	customModelRepo := custommodel.NewRepository(db.GetDB())
	customModelService := custommodel.NewService(customModelRepo, secretBox)
	customModelHandler := custommodel.NewHandler(customModelService)

	chatHandler := chat.NewHandler(conversationService, sshconnService, hasAccessLevel, sshCommandPolicy, providerKeyService, customModelService)

	engine, v1 := httpserver.NewEngine(logger)

	authMiddleware := authService.Authentication()

	user.RegisterRoutes(v1, userHandler, authMiddleware, hasAccessLevel)
	rbac.RegisterRoutes(v1, rbacHandler, authMiddleware, rbacService)
	sshconn.RegisterRoutes(v1, sshconnHandler, authMiddleware, hasAccessLevel)
	providerkey.RegisterRoutes(v1, providerKeyHandler, authMiddleware, hasAccessLevel)
	custommodel.RegisterRoutes(v1, customModelHandler, authMiddleware, hasAccessLevel)
	chat.RegisterRoutes(v1, chatHandler, authMiddleware)
	auth.RegisterRoutes(v1, authHandler)

	logger.Info(logging.General, logging.Startup, "starting vohu server", nil)

	if err := httpserver.Run(engine, cfg); err != nil {
		logger.Fatal(err, logging.General, logging.Startup, err.Error(), nil)
	}
}
