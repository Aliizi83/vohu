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

	rbacRepo := rbac.NewRepository(db)
	rbacService := rbac.NewService(rbacRepo)
	rbacHandler := rbac.NewHandler(rbacService)

	hasAccessLevel := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return rbacService.HasAccessLevel(ctx, userID, resourceType, resourceID, rbac.AccessLevel(level))
	}

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo, hasAccessLevel)
	userHandler := user.NewHandler(userService)

	authService := auth.NewService(cfg, userService)
	authHandler := auth.NewHandler(authService)

	engine, v1 := httpserver.NewEngine(logger)
	authMiddleware := authService.Authentication()

	user.RegisterRoutes(v1, userHandler, authMiddleware, hasAccessLevel)
	rbac.RegisterRoutes(v1, rbacHandler, authMiddleware, rbacService)
	auth.RegisterRoutes(v1, authHandler)

	// Seed an admin with a wildcard "manage" grant on every known resource
	// type (manage on shared.WildcardResourceID satisfies any real ID,
	// including ones created after this grant), and a plain user with no
	// grants at all — same shape as cmd/server/seeders' seedAdminAccess,
	// built directly through the services here so the test doesn't depend
	// on the seeders package's specific dev credentials.
	if _, err := userService.Register(ctx, user.CreateUserRequest{Username: "admin", Password: "adminpass123"}); err != nil {
		t.Fatalf("seed admin failed: %v", err)
	}
	admin, err := userService.GetByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("get seeded admin failed: %v", err)
	}

	for _, resourceType := range rbac.KnownResourceTypes {
		if err := rbacService.GrantResourceAccess(
			ctx, rbac.GranteeUser, admin.ID, resourceType, shared.WildcardResourceID, rbac.AccessManage, rbac.EffectAccepted,
		); err != nil {
			t.Fatalf("grant admin wildcard access for %q failed: %v", resourceType, err)
		}
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

// createUser registers a user through the HTTP API (as adminToken, who
// holds wildcard "manage" on "user") and returns its ID.
func createUser(t *testing.T, engine *gin.Engine, adminToken, username string) uint {
	t.Helper()
	rec := doJSON(t, engine, http.MethodPost, "/api/v1/users", adminToken, map[string]string{
		"username": username,
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating user %q failed: status=%d body=%s", username, rec.Code, rec.Body.String())
	}
	var created struct {
		Result struct {
			ID uint `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create-user response: %v", err)
	}
	return created.Result.ID
}

func createRole(t *testing.T, engine *gin.Engine, adminToken, name string) uint {
	t.Helper()
	rec := doJSON(t, engine, http.MethodPost, "/api/v1/roles", adminToken, map[string]string{"name": name})
	if rec.Code != http.StatusCreated {
		t.Fatalf("creating role %q failed: status=%d body=%s", name, rec.Code, rec.Body.String())
	}
	var created struct {
		Result struct {
			ID uint `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal create-role response: %v", err)
	}
	return created.Result.ID
}

func assignRole(t *testing.T, engine *gin.Engine, adminToken string, userID, roleID uint) {
	t.Helper()
	path := "/api/v1/users/" + strconv.FormatUint(uint64(userID), 10) + "/roles"
	rec := doJSON(t, engine, http.MethodPost, path, adminToken, map[string]uint{"roleId": roleID})
	if rec.Code != http.StatusOK {
		t.Fatalf("assigning role %d to user %d failed: status=%d body=%s", roleID, userID, rec.Code, rec.Body.String())
	}
}

func grantResourceAccess(
	t *testing.T, engine *gin.Engine, callerToken string,
	granteeType string, granteeID uint, resourceType string, resourceID uint, level, effect string,
) *httptest.ResponseRecorder {
	t.Helper()
	return doJSON(t, engine, http.MethodPost, "/api/v1/resource-access", callerToken, map[string]any{
		"granteeType":  granteeType,
		"granteeId":    granteeID,
		"resourceType": resourceType,
		"resourceId":   resourceID,
		"level":        level,
		"effect":       effect,
	})
}

// TestIntegration_ResourceAccessRoleCascade_HTTP drives the "a role can be
// granted access to a user resource, cascading to every member of that
// role, except a per-user prohibited override" behavior end to end
// through the real router — role.Service.HasAccessLevel's cascade and
// shared.RequireAccessLevelOnParam's route wiring are already covered in
// isolation (rbac/service_test.go, sshconn/service_test.go), this is the
// same scenario exercised the way an actual client would: real HTTP
// requests, real JWTs, real gin routing.
func TestIntegration_ResourceAccessRoleCascade_HTTP(t *testing.T) {
	engine := setupIntegrationServer(t)
	adminToken := loginAndGetToken(t, engine, "admin", "adminpass123")

	reviewerRoleID := createRole(t, engine, adminToken, "reviewer")

	reviewerID := createUser(t, engine, adminToken, "reviewer1")
	member1ID := createUser(t, engine, adminToken, "member1")
	member2ID := createUser(t, engine, adminToken, "member2")
	outsiderID := createUser(t, engine, adminToken, "outsider")

	assignRole(t, engine, adminToken, member1ID, reviewerRoleID)
	assignRole(t, engine, adminToken, member2ID, reviewerRoleID)

	reviewerToken := loginAndGetToken(t, engine, "reviewer1", "password123")

	member1Path := "/api/v1/users/" + strconv.FormatUint(uint64(member1ID), 10)
	member2Path := "/api/v1/users/" + strconv.FormatUint(uint64(member2ID), 10)
	outsiderPath := "/api/v1/users/" + strconv.FormatUint(uint64(outsiderID), 10)

	// Before any grant, the reviewer has no visibility into anyone.
	if rec := doJSON(t, engine, http.MethodGet, member1Path, reviewerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 before any grant, got %d: %s", rec.Code, rec.Body.String())
	}

	// A non-manage caller can't grant access at all.
	if rec := grantResourceAccess(t, engine, reviewerToken, "user", reviewerID, "role", reviewerRoleID, "read", "accepted"); rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 granting access without manage rights, got %d: %s", rec.Code, rec.Body.String())
	}

	// Admin grants the reviewer direct read access to the "reviewer" role
	// as a resource — this is what makes HasAccessLevel's cascade reach
	// every member of that role.
	if rec := grantResourceAccess(t, engine, adminToken, "user", reviewerID, "role", reviewerRoleID, "read", "accepted"); rec.Code != http.StatusOK {
		t.Fatalf("granting role access failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, engine, http.MethodGet, member1Path, reviewerToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for a role member after the cascade grant, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, engine, http.MethodGet, member2Path, reviewerToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for the other role member too, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, engine, http.MethodGet, outsiderPath, reviewerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for a user outside the role, got %d: %s", rec.Code, rec.Body.String())
	}

	// A more specific, direct "prohibited" row on member2 carves them back
	// out of the cascade while member1 stays visible.
	if rec := grantResourceAccess(t, engine, adminToken, "user", reviewerID, "user", member2ID, "read", "prohibited"); rec.Code != http.StatusOK {
		t.Fatalf("granting the prohibited override failed: status=%d body=%s", rec.Code, rec.Body.String())
	}

	if rec := doJSON(t, engine, http.MethodGet, member2Path, reviewerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for member2 after the prohibited override, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, engine, http.MethodGet, member1Path, reviewerToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("expected member1 to remain visible after member2's override, got %d: %s", rec.Code, rec.Body.String())
	}

	// GET /me/access reflects the caller's own grants and computed levels.
	rec := doJSON(t, engine, http.MethodGet, "/api/v1/me/access", reviewerToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /me/access failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var access struct {
		Result struct {
			ResourceAccess []struct {
				GranteeType  string `json:"granteeType"`
				ResourceType string `json:"resourceType"`
				Effect       string `json:"effect"`
			} `json:"resourceAccess"`
			Levels map[string]string `json:"levels"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &access); err != nil {
		t.Fatalf("failed to unmarshal /me/access response: %v", err)
	}
	if len(access.Result.ResourceAccess) != 2 {
		t.Fatalf("expected 2 grants belonging to the reviewer (role-read + prohibited-override), got %d: %+v",
			len(access.Result.ResourceAccess), access.Result.ResourceAccess)
	}
	// The reviewer never held a grant checked against the wildcard "user"
	// resource ID (only real, specific IDs) so /me/access should report no
	// blanket level for it — cascade access to specific users isn't the
	// same thing as a level on the type at large.
	if level, ok := access.Result.Levels["user"]; ok {
		t.Fatalf("expected no wildcard-level entry for \"user\", got %q", level)
	}

	// Revoking the role-level grant removes the reviewer's cascade access
	// entirely, including to member1.
	revokeID := findResourceAccessID(t, engine, adminToken, "role", reviewerRoleID)
	if rec := doJSON(t, engine, http.MethodDelete, "/api/v1/resource-access/"+strconv.FormatUint(uint64(revokeID), 10), adminToken, nil); rec.Code != http.StatusOK {
		t.Fatalf("revoking the role grant failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, engine, http.MethodGet, member1Path, reviewerToken, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for member1 after revoking the cascade grant, got %d: %s", rec.Code, rec.Body.String())
	}
}

// findResourceAccessID looks up the auto-generated ID of the resource_access
// row granted to (user, reviewerID) on ("role", roleID) — the grant/revoke
// HTTP API never hands the caller an ID back on creation (POST just
// upserts), so a caller wanting to revoke a specific grant has to look it
// up via the list endpoint first, the same way the frontend does.
func findResourceAccessID(t *testing.T, engine *gin.Engine, adminToken string, resourceType string, resourceID uint) uint {
	t.Helper()
	rec := doJSON(t, engine, http.MethodGet, "/api/v1/resource-access?pageSize=100", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("listing resource access failed: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Result struct {
			Items []struct {
				ID           uint   `json:"id"`
				ResourceType string `json:"resourceType"`
				ResourceID   uint   `json:"resourceId"`
			} `json:"items"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("failed to unmarshal resource-access list: %v", err)
	}
	for _, item := range listed.Result.Items {
		if item.ResourceType == resourceType && item.ResourceID == resourceID {
			return item.ID
		}
	}
	t.Fatalf("no resource_access row found for (%s, %d)", resourceType, resourceID)
	return 0
}
