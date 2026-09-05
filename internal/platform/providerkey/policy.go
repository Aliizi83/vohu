package providerkey

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// Policy only gates the global-key routes — the "my key" routes are
// self-service (any authenticated user manages their own), same as
// conversation ownership needing no separate policy.
type Policy struct {
	hasPermission shared.PermissionCheck
}

func NewPolicy(hasPermission shared.PermissionCheck) *Policy {
	return &Policy{hasPermission: hasPermission}
}

func (p *Policy) CanManageGlobal(ctx context.Context, userID uint) (bool, error) {
	return p.hasPermission(ctx, userID, "providerkeys:manage")
}
