package rbac

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var ErrRoleExists = errors.New("role already exists")

// Service is what every other module depends on — never Repository
// directly. HasAccessLevel is the platform's one authorization primitive;
// everything else here manages the Role/ResourceAccess data it resolves
// against.
type Service interface {
	CreateRole(ctx context.Context, req CreateRoleRequest) (*Role, error)
	GetRole(ctx context.Context, id uint) (*Role, error)
	UpdateRole(ctx context.Context, id uint, req UpdateRoleRequest) (*Role, error)
	DeleteRole(ctx context.Context, id uint) error
	ListRoles(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]Role, int64, error)
	AssignRoleToUser(ctx context.Context, userID, roleID uint) error

	// GrantResourceAccess records a level+effect for one grantee (a
	// specific user, or every member of a role) on one (resourceType,
	// resourceID) pair — an upsert, a repeat grant just updates it.
	GrantResourceAccess(ctx context.Context, granteeType GranteeType, granteeID uint, resourceType string, resourceID uint, level AccessLevel, effect ResourceEffect) error
	GetResourceAccess(ctx context.Context, id uint) (*ResourceAccess, error)
	RevokeResourceAccess(ctx context.Context, id uint) error
	ListResourceAccess(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]ResourceAccess, int64, error)

	// HasAccessLevel is the resolver — see its doc comment for the full
	// algorithm. Every route in the platform is gated on a call to this,
	// directly or via shared.RequireAccessLevelOnParam/Wildcard.
	HasAccessLevel(ctx context.Context, userID uint, resourceType string, resourceID uint, required AccessLevel) (bool, error)

	// MyResourceAccess and MyLevel back the self-service /me/access
	// endpoint — a user's own complete access profile, computed without
	// needing to know every resource ID up front.
	MyResourceAccess(ctx context.Context, userID uint) ([]ResourceAccess, error)
	MyLevel(ctx context.Context, userID uint, resourceType string) (level AccessLevel, ok bool, err error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreateRole(ctx context.Context, req CreateRoleRequest) (*Role, error) {
	_, err := s.repo.FindRoleByName(ctx, req.Name)
	if err == nil {
		return nil, ErrRoleExists
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return nil, err
	}

	role := &Role{Name: req.Name}
	if err := s.repo.CreateRole(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *service) GetRole(ctx context.Context, id uint) (*Role, error) {
	return s.repo.FindRoleByID(ctx, id)
}

func (s *service) UpdateRole(ctx context.Context, id uint, req UpdateRoleRequest) (*Role, error) {
	role, err := s.repo.FindRoleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		role.Name = req.Name
	}

	if err := s.repo.UpdateRole(ctx, role); err != nil {
		return nil, err
	}

	return role, nil
}

func (s *service) DeleteRole(ctx context.Context, id uint) error {
	return s.repo.DeleteRole(ctx, id)
}

func (s *service) ListRoles(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]Role, int64, error) {
	return s.repo.ListRoles(ctx, filter, page)
}

func (s *service) AssignRoleToUser(ctx context.Context, userID, roleID uint) error {
	exists, err := s.repo.UserRoleExists(ctx, userID, roleID)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	return s.repo.AssignRoleToUser(ctx, userID, roleID)
}

func (s *service) GrantResourceAccess(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
	level AccessLevel,
	effect ResourceEffect,
) error {
	return s.repo.UpsertResourceAccess(ctx, granteeType, granteeID, resourceType, resourceID, level, effect)
}

func (s *service) GetResourceAccess(ctx context.Context, id uint) (*ResourceAccess, error) {
	return s.repo.FindResourceAccessByID(ctx, id)
}

func (s *service) RevokeResourceAccess(ctx context.Context, id uint) error {
	return s.repo.DeleteResourceAccess(ctx, id)
}

func (s *service) ListResourceAccess(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]ResourceAccess, int64, error) {
	return s.repo.ListResourceAccess(ctx, filter, page)
}

// HasAccessLevel resolves, most-specific-wins, prohibited-is-a-hard-veto:
//
//  1. The exact row for (user, userID, resourceType, resourceID), if one
//     exists, is authoritative — stop, whatever it says.
//  2. Else the wildcard row for the same grantee (resourceID replaced
//     with shared.WildcardResourceID) — same, authoritative if present.
//  3. Else every role userID holds: exact-then-wildcard per role, same as
//     above. Aggregated across all the user's roles — if *any* role row
//     is prohibited, deny outright even if another role would have
//     granted it; else allow if *any* role row accepts sufficiently; else
//     deny if any role row existed at all but none sufficed.
//  4. Else, only when resourceType == "user": recurse into resourceType
//     "role" for every role the *target* user (resourceID) holds — "does
//     the caller have required-level access to that role as a resource."
//     This is the cascade: a grant on a role reaches every member of that
//     role, and step 1/2 above (checked first, on the specific target
//     user) is exactly what lets an explicit prohibited row carve one
//     member back out of that cascade.
//  5. Otherwise, default deny.
func (s *service) HasAccessLevel(
	ctx context.Context,
	userID uint,
	resourceType string,
	resourceID uint,
	required AccessLevel,
) (bool, error) {
	res, err := s.findGrant(ctx, GranteeUser, userID, resourceType, resourceID)
	if err != nil {
		return false, err
	}
	if res.found {
		return res.effect == EffectAccepted && res.level.Satisfies(required), nil
	}

	roleIDs, err := s.repo.GetRoleIDsForUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if len(roleIDs) > 0 {
		allowed, decided, err := s.resolveAnyRole(ctx, roleIDs, resourceType, resourceID, required)
		if err != nil {
			return false, err
		}
		if decided {
			return allowed, nil
		}
	}

	if resourceType == "user" {
		targetRoleIDs, err := s.repo.GetRoleIDsForUser(ctx, resourceID)
		if err != nil {
			return false, err
		}
		for _, roleID := range targetRoleIDs {
			ok, err := s.HasAccessLevel(ctx, userID, "role", roleID, required)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}

	return false, nil
}

type grantResolution struct {
	found  bool
	effect ResourceEffect
	level  AccessLevel
}

// findGrant looks up one grantee's exact row for (resourceType,
// resourceID), falling back to that same grantee's wildcard row
// (shared.WildcardResourceID) if the exact one doesn't exist.
// found=false means this grantee has no applicable row at all.
func (s *service) findGrant(
	ctx context.Context,
	granteeType GranteeType,
	granteeID uint,
	resourceType string,
	resourceID uint,
) (grantResolution, error) {
	row, err := s.repo.FindResourceAccess(ctx, granteeType, granteeID, resourceType, resourceID)
	if err == nil {
		return grantResolution{found: true, effect: row.Effect, level: row.Level}, nil
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return grantResolution{}, err
	}

	if resourceID == shared.WildcardResourceID {
		return grantResolution{}, nil
	}

	row, err = s.repo.FindResourceAccess(ctx, granteeType, granteeID, resourceType, shared.WildcardResourceID)
	if err == nil {
		return grantResolution{found: true, effect: row.Effect, level: row.Level}, nil
	}
	if !errors.Is(err, shared.ErrNotFound) {
		return grantResolution{}, err
	}

	return grantResolution{}, nil
}

// resolveAnyRole aggregates findGrant across every role a user holds: an
// explicit prohibited row from any one role vetoes the whole check, even
// if another role would otherwise have granted it sufficiently.
func (s *service) resolveAnyRole(
	ctx context.Context,
	roleIDs []uint,
	resourceType string,
	resourceID uint,
	required AccessLevel,
) (allowed bool, decided bool, err error) {
	sawRow := false
	sawSufficientAccept := false

	for _, roleID := range roleIDs {
		res, err := s.findGrant(ctx, GranteeRole, roleID, resourceType, resourceID)
		if err != nil {
			return false, false, err
		}
		if !res.found {
			continue
		}
		sawRow = true

		if res.effect == EffectProhibited {
			return false, true, nil
		}
		if res.effect == EffectAccepted && res.level.Satisfies(required) {
			sawSufficientAccept = true
		}
	}

	if sawSufficientAccept {
		return true, true, nil
	}
	if sawRow {
		return false, true, nil
	}
	return false, false, nil
}

func (s *service) MyResourceAccess(ctx context.Context, userID uint) ([]ResourceAccess, error) {
	direct, err := s.repo.ListResourceAccessForGrantee(ctx, GranteeUser, userID)
	if err != nil {
		return nil, err
	}

	roleIDs, err := s.repo.GetRoleIDsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	result := direct
	for _, roleID := range roleIDs {
		viaRole, err := s.repo.ListResourceAccessForGrantee(ctx, GranteeRole, roleID)
		if err != nil {
			return nil, err
		}
		result = append(result, viaRole...)
	}

	return result, nil
}

// MyLevel reports the best (highest) level userID holds on resourceType
// at large (checked against shared.WildcardResourceID) — what /me/access
// uses to tell the frontend "can I see the Users nav item at all," not a
// check against any one specific row.
func (s *service) MyLevel(ctx context.Context, userID uint, resourceType string) (AccessLevel, bool, error) {
	for _, level := range []AccessLevel{AccessManage, AccessWrite, AccessRead} {
		ok, err := s.HasAccessLevel(ctx, userID, resourceType, shared.WildcardResourceID, level)
		if err != nil {
			return "", false, err
		}
		if ok {
			return level, true, nil
		}
	}
	return "", false, nil
}
