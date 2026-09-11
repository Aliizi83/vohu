package rbac_test

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestCachedRepo(t *testing.T) (rbac.Repository, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { client.Close() })

	inner := rbac.NewRepository(setupRBACTestDB(t))
	return rbac.NewCachedRepository(inner, client), mr
}

// TestCachedRepository_FindResourceAccess_CacheHitReturnsCorrectRow checks
// the read-through path returns the right row both on the cold (DB) read
// and the warm (cache) read that follows it.
func TestCachedRepository_FindResourceAccess_CacheHitReturnsCorrectRow(t *testing.T) {
	repo, _ := newTestCachedRepo(t)
	ctx := context.Background()

	if err := repo.UpsertResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7, rbac.AccessWrite, rbac.EffectAccepted); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	for i, label := range []string{"cold", "warm"} {
		_ = i
		row, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7)
		if err != nil {
			t.Fatalf("%s FindResourceAccess failed: %v", label, err)
		}
		if row.Level != rbac.AccessWrite || row.Effect != rbac.EffectAccepted {
			t.Fatalf("%s read returned wrong row: %+v", label, row)
		}
	}
}

// TestCachedRepository_UpsertInvalidatesCache is the correctness-critical
// case: a cached grantee list must not keep serving pre-grant data after a
// new grant is written, or access would silently lag its own database.
func TestCachedRepository_UpsertInvalidatesCache(t *testing.T) {
	repo, _ := newTestCachedRepo(t)
	ctx := context.Background()

	// Warm the cache with "no grant".
	if _, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound before grant, got %v", err)
	}

	if err := repo.UpsertResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	row, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7)
	if err != nil {
		t.Fatalf("expected the new grant to be visible immediately, got err: %v", err)
	}
	if row.Level != rbac.AccessManage {
		t.Fatalf("expected AccessManage, got %v", row.Level)
	}
}

// TestCachedRepository_DeleteInvalidatesCache is the mirror case: a
// revoked grant must stop being served the moment it's revoked, not after
// the TTL — a stale-cache-allows-access bug is a security bug.
func TestCachedRepository_DeleteInvalidatesCache(t *testing.T) {
	repo, _ := newTestCachedRepo(t)
	ctx := context.Background()

	if err := repo.UpsertResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7, rbac.AccessManage, rbac.EffectAccepted); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Warm the cache with the grant present.
	if _, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7); err != nil {
		t.Fatalf("expected the grant to resolve before revoke: %v", err)
	}

	grant, err := repo.FindResourceAccessByID(ctx, 1)
	if err != nil {
		t.Fatalf("FindResourceAccessByID failed: %v", err)
	}

	if err := repo.DeleteResourceAccess(ctx, grant.ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	if _, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7); err != shared.ErrNotFound {
		t.Fatalf("expected ErrNotFound immediately after revoke, got %v", err)
	}
}

// TestCachedRepository_AssignRoleInvalidatesRoleCache checks the role-IDs
// cache is invalidated on assignment rather than only expiring by TTL.
func TestCachedRepository_AssignRoleInvalidatesRoleCache(t *testing.T) {
	repo, _ := newTestCachedRepo(t)
	ctx := context.Background()

	role := &rbac.Role{Name: "operator"}
	if err := repo.CreateRole(ctx, role); err != nil {
		t.Fatalf("create role failed: %v", err)
	}

	// Warm the cache with "no roles".
	roleIDs, err := repo.GetRoleIDsForUser(ctx, 1)
	if err != nil {
		t.Fatalf("GetRoleIDsForUser failed: %v", err)
	}
	if len(roleIDs) != 0 {
		t.Fatalf("expected no roles yet, got %v", roleIDs)
	}

	if err := repo.AssignRoleToUser(ctx, 1, role.ID); err != nil {
		t.Fatalf("assign role failed: %v", err)
	}

	roleIDs, err = repo.GetRoleIDsForUser(ctx, 1)
	if err != nil {
		t.Fatalf("GetRoleIDsForUser after assign failed: %v", err)
	}
	if len(roleIDs) != 1 || roleIDs[0] != role.ID {
		t.Fatalf("expected [%d], got %v", role.ID, roleIDs)
	}
}

// TestCachedRepository_FailsOpenWhenRedisIsDown checks that a broken Redis
// connection degrades to hitting the database directly rather than making
// every access check fail.
func TestCachedRepository_FailsOpenWhenRedisIsDown(t *testing.T) {
	repo, mr := newTestCachedRepo(t)
	ctx := context.Background()

	if err := repo.UpsertResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7, rbac.AccessRead, rbac.EffectAccepted); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	mr.Close()

	row, err := repo.FindResourceAccess(ctx, rbac.GranteeUser, 1, "ssh_connection", 7)
	if err != nil {
		t.Fatalf("expected fail-open to the DB, got err: %v", err)
	}
	if row.Level != rbac.AccessRead {
		t.Fatalf("expected AccessRead, got %v", row.Level)
	}
}
