package rbac

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/cache"
	"github.com/redis/go-redis/v9"
)

// accessCacheTTL bounds how long a grantee's cached ResourceAccess rows (or
// a user's cached role IDs) can outlive the write that should have
// invalidated them, in case an invalidation call is ever missed —
// invalidation on every write (below) is the actual correctness mechanism,
// this is only the safety net under it.
const accessCacheTTL = 5 * time.Minute

// cachedRepository wraps a Repository with a Redis read-through cache over
// its two hottest reads — FindResourceAccess/ListResourceAccessForGrantee
// and GetRoleIDsForUser — both hit on essentially every authenticated
// request via Service.HasAccessLevel. Every other method passes straight
// through: role CRUD and the paginated/admin-facing ResourceAccess listing
// aren't on that hot path.
//
// Caching is keyed per grantee (every ResourceAccess row for one user or
// role), not per (resourceType, resourceID, level) query — one key to
// invalidate on any write touching that grantee, and both the exact-ID and
// wildcard-ID lookups a single HasAccessLevel call makes are served from
// the same cached list.
//
// If Redis is unreachable, reads fall through to inner rather than failing
// the request — a broken cache must only make authorization slower, never
// turn into "nobody can do anything."
type cachedRepository struct {
	inner Repository
	redis *redis.Client
}

func NewCachedRepository(inner Repository, redisClient *redis.Client) Repository {
	return &cachedRepository{inner: inner, redis: redisClient}
}

func granteeAccessKey(granteeType GranteeType, granteeID uint) string {
	return fmt.Sprintf("rbac:access:%s:%d", granteeType, granteeID)
}

func userRolesKey(userID uint) string {
	return fmt.Sprintf("rbac:roles:user:%d", userID)
}

// granteeAccess is the read-through core: every ResourceAccess row for one
// grantee, cache-first.
func (r *cachedRepository) granteeAccess(ctx context.Context, granteeType GranteeType, granteeID uint) ([]ResourceAccess, error) {
	key := granteeAccessKey(granteeType, granteeID)

	cached, err := cache.Get[[]ResourceAccess](ctx, r.redis, key)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		// Redis itself is unreachable/broken, not just a miss — fail open
		// to the DB rather than the request.
		return r.inner.ListResourceAccessForGrantee(ctx, granteeType, granteeID)
	}

	rows, err := r.inner.ListResourceAccessForGrantee(ctx, granteeType, granteeID)
	if err != nil {
		return nil, err
	}
	_ = cache.Set(ctx, r.redis, key, rows, accessCacheTTL)
	return rows, nil
}

func (r *cachedRepository) invalidateGrantee(ctx context.Context, granteeType GranteeType, granteeID uint) {
	r.redis.Del(ctx, granteeAccessKey(granteeType, granteeID))
}

func (r *cachedRepository) FindResourceAccess(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
) (*ResourceAccess, error) {
	rows, err := r.granteeAccess(ctx, granteeType, granteeID)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		if rows[i].ResourceType == resourceType && rows[i].ResourceID == resourceID {
			row := rows[i]
			return &row, nil
		}
	}
	return nil, shared.ErrNotFound
}

func (r *cachedRepository) ListResourceAccessForGrantee(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
) ([]ResourceAccess, error) {
	return r.granteeAccess(ctx, granteeType, granteeID)
}

func (r *cachedRepository) GetRoleIDsForUser(ctx context.Context, userID uint) ([]uint, error) {
	key := userRolesKey(userID)

	cached, err := cache.Get[[]uint](ctx, r.redis, key)
	if err == nil {
		return cached, nil
	}
	if !errors.Is(err, redis.Nil) {
		return r.inner.GetRoleIDsForUser(ctx, userID)
	}

	roleIDs, err := r.inner.GetRoleIDsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	_ = cache.Set(ctx, r.redis, key, roleIDs, accessCacheTTL)
	return roleIDs, nil
}

func (r *cachedRepository) UpsertResourceAccess(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
	level AccessLevel,
	effect ResourceEffect,
) error {
	if err := r.inner.UpsertResourceAccess(ctx, granteeType, granteeID, resourceType, resourceID, level, effect); err != nil {
		return err
	}
	r.invalidateGrantee(ctx, granteeType, granteeID)
	return nil
}

func (r *cachedRepository) DeleteResourceAccess(ctx context.Context, id uint) error {
	// The grantee has to be learned before the row is gone — nothing left
	// to look up by id afterward.
	row, findErr := r.inner.FindResourceAccessByID(ctx, id)

	if err := r.inner.DeleteResourceAccess(ctx, id); err != nil {
		return err
	}
	if findErr == nil {
		r.invalidateGrantee(ctx, row.GranteeType, row.GranteeID)
	}
	return nil
}

func (r *cachedRepository) AssignRoleToUser(ctx context.Context, userID, roleID uint) error {
	if err := r.inner.AssignRoleToUser(ctx, userID, roleID); err != nil {
		return err
	}
	r.redis.Del(ctx, userRolesKey(userID))
	return nil
}

// Everything below passes straight through — role CRUD and the
// paginated/admin-facing listing aren't on Service.HasAccessLevel's hot
// path, so there's nothing worth caching here.

func (r *cachedRepository) CreateRole(ctx context.Context, role *Role) error {
	return r.inner.CreateRole(ctx, role)
}

func (r *cachedRepository) FindRoleByID(ctx context.Context, id uint) (*Role, error) {
	return r.inner.FindRoleByID(ctx, id)
}

func (r *cachedRepository) FindRoleByName(ctx context.Context, name string) (*Role, error) {
	return r.inner.FindRoleByName(ctx, name)
}

func (r *cachedRepository) UpdateRole(ctx context.Context, role *Role) error {
	return r.inner.UpdateRole(ctx, role)
}

func (r *cachedRepository) DeleteRole(ctx context.Context, id uint) error {
	return r.inner.DeleteRole(ctx, id)
}

func (r *cachedRepository) ListRoles(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Role, int64, error) {
	return r.inner.ListRoles(ctx, filter, page)
}

func (r *cachedRepository) UserRoleExists(ctx context.Context, userID, roleID uint) (bool, error) {
	return r.inner.UserRoleExists(ctx, userID, roleID)
}

func (r *cachedRepository) FindResourceAccessByID(ctx context.Context, id uint) (*ResourceAccess, error) {
	return r.inner.FindResourceAccessByID(ctx, id)
}

func (r *cachedRepository) ListResourceAccess(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]ResourceAccess, int64, error) {
	return r.inner.ListResourceAccess(ctx, filter, page)
}
