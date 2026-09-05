package sshconn

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// Policy gates the CRUD routes below — "who may manage connection rows at
// all" (flat permission keys, same shape as user.Policy). This is
// deliberately separate from rbac.ResourcePermission, which answers a
// narrower question — "can this user actually use *this* connection" —
// checked directly by the chat module's SSHTool before it ever dials, not
// here.
type Policy struct {
	hasPermission shared.PermissionCheck
}

func NewPolicy(hasPermission shared.PermissionCheck) *Policy {
	return &Policy{hasPermission: hasPermission}
}

func (p *Policy) CanCreate(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "ssh:create")
}

func (p *Policy) CanRead(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "ssh:read")
}

func (p *Policy) CanUpdate(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "ssh:update")
}

func (p *Policy) CanDelete(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "ssh:delete")
}
