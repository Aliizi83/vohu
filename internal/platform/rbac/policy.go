package rbac

import "context"

// Policy decides what an authenticated user may do to Role/Permission
// resources. Unlike other modules' policies, this one doesn't need a
// checker injected — rbac already owns HasPermission itself.
type Policy struct {
	service Service
}

func NewPolicy(service Service) *Policy {
	return &Policy{service: service}
}

// CanManage covers create/update/delete/grant/assign for both roles and
// permissions — the admin surface isn't split into per-action permissions
// yet (see seeders.KnownPermissions).
func (p *Policy) CanManage(ctx context.Context, userID uint) (bool, error) {
	return p.service.HasPermission(ctx, userID, "rbac:manage")
}
