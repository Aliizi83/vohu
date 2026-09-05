package user

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// Policy decides what an authenticated user may do to User resources.
// Constructed with rbac's permission checker injected — same DI style as
// RegisterRoutes — so this module never imports rbac directly. Each method
// is a flat permission-key lookup today; this is the seam for logic
// beyond that later (e.g. "a user can read their own record without
// user:read") without changing how routes.go calls it.
type Policy struct {
	hasPermission shared.PermissionCheck
}

func NewPolicy(hasPermission shared.PermissionCheck) *Policy {
	return &Policy{hasPermission: hasPermission}
}

func (p *Policy) CanCreate(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "user:create")
}

func (p *Policy) CanRead(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "user:read")
}

func (p *Policy) CanUpdate(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "user:update")
}

func (p *Policy) CanDelete(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "user:delete")
}
