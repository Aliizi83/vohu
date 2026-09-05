// Package platform_test wires the same modules cmd/server/main.go does
// (user, rbac, auth, httpserver) against an in-memory SQLite DB and drives
// the assembled router with httptest — the same login -> 401/403/200 ->
// update -> delete -> 404 flow verified manually via curl earlier this
// session, now as a regression test.
package platform_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/Aliizi83/vohu/config"
	"github.com/Aliizi83/vohu/internal/platform/auth"
	"github.com/Aliizi83/vohu/internal/platform/httpserver"
	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"github.com/Aliizi83/vohu/pkg/logging"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupIntegrationServer(t *testing.T) *gin.Engine {
	t.Helper()
	ctx := context.Background()

	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(shared.AllModels...); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	cfg := &config.Config{
		Logger: config.LoggerConfig{FileFolderPath: t.TempDir(), Level: "error", Logger: "zap"},
		JWT: config.JWTConfig{
			Secret:                     "test-secret",
			RefreshSecret:              "test-refresh-secret",
			AccessTokenExpireDuration:  15,
			RefreshTokenExpireDuration: 60,
		},
	}
	logger := logging.NewLogger(cfg)

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	rbacRepo := rbac.NewRepository(db)
	rbacService := rbac.NewService(rbacRepo)
	rbacHandler := rbac.NewHandler(rbacService)
	rbacPolicy := rbac.NewPolicy(rbacService)

	userPolicy := user.NewPolicy(rbacService.HasPermission)

	authService := auth.NewService(cfg, userService)
	authHandler := auth.NewHandler(authService)

	engine, v1 := httpserver.NewEngine(logger)
	authMiddleware := authService.Authentication()

	user.RegisterRoutes(v1, userHandler, authMiddleware, userPolicy)
	rbac.RegisterRoutes(v1, rbacHandler, authMiddleware, rbacPolicy)
	auth.RegisterRoutes(v1, authHandler)

	// Seed an admin with every user permission, and a plain user with none
	// — same shape as cmd/server/seeders, built directly through the
	// services here so the test doesn't depend on the seeders package's
	// specific dev credentials.
	if _, err := userService.Register(ctx, user.CreateUserRequest{Username: "admin", Password: "adminpass123"}); err != nil {
		t.Fatalf("seed admin failed: %v", err)
	}
	admin, err := userService.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("get seeded admin failed: %v", err)
	}

	role, err := rbacService.CreateRole(ctx, rbac.CreateRoleRequest{Name: "admin"})
	if err != nil {
		t.Fatalf("create role failed: %v", err)
	}

	for _, key := range []string{"user:create", "user:read", "user:update", "user:delete", "rbac:manage"} {
		p, err := rbacService.CreatePermission(ctx, rbac.CreatePermissionRequest{Key: key})
		if err != nil {
			t.Fatalf("create permission %q failed: %v", key, err)
		}
		if err := rbacService.GrantPermissionToRole(ctx, role.ID, p.ID); err != nil {
			t.Fatalf("grant %q failed: %v", key, err)
		}
	}
	if err := rbacService.AssignRoleToUser(ctx, admin.ID, role.ID); err != nil {
		t.Fatalf("assign role to admin failed: %v", err)
	}

	if _, err := userService.Register(ctx, user.CreateUserRequest{Username: "plain", Password: "plainpass123"}); err != nil {
		t.Fatalf("seed plain user failed: %v", err)
	}

	return engine
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body failed: %v", err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func loginAndGetToken(t *testing.T, engine *gin.Engine, username, password string) string {
	t.Helper()

	rec := doJSON(t, engine, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"username": username,
		"password": password,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("login failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			AccessToken string `json:"accessToken"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal login response: %v", err)
	}
	if resp.Result.AccessToken == "" {
		t.Fatalf("expected a non-empty accessToken, got body: %s", rec.Body.String())
	}

	return resp.Result.AccessToken
}

func TestIntegration_FullUserLifecycle(t *testing.T) {
	engine := setupIntegrationServer(t)

	if rec := doJSON(t, engine, http.MethodGet, "/api/v1/users/1", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for an unauthenticated request, got %d: %s", rec.Code, rec.Body.String())
	}

	adminToken := loginAndGetToken(t, engine, "admin", "adminpass123")
	plainToken := loginAndGetToken(t, engine, "plain", "plainpass123")

	if rec := doJSON(t, engine, http.MethodPost, "/api/v1/users", plainToken, map[string]string{
		"username": "should-fail",
		"password": "password123",
	}); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a user with no permissions, got %d: %s", rec.Code, rec.Body.String())
	}

	rec := doJSON(t, engine, http.MethodPost, "/api/v1/users", adminToken, map[string]string{
		"username": "newuser",
		"email":    "newuser@example.com",
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 creating a user, got %d: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		Result struct {
			ID uint `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}
	if created.Result.ID == 0 {
		t.Fatalf("expected a nonzero created user id, got body: %s", rec.Body.String())
	}
	newID := strconv.FormatUint(uint64(created.Result.ID), 10)

	if rec := doJSON(t, engine, http.MethodPut, "/api/v1/users/"+newID, adminToken, map[string]any{"enabled": false}); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 updating a user, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, engine, http.MethodDelete, "/api/v1/users/"+newID, adminToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 deleting a user, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, engine, http.MethodGet, "/api/v1/users/"+newID, adminToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d: %s", rec.Code, rec.Body.String())
	}
}
