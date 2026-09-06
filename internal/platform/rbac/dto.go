package rbac

type CreateRoleRequest struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

type UpdateRoleRequest struct {
	Name string `json:"name" binding:"omitempty,min=2,max=50"`
}

type AssignRoleRequest struct {
	RoleID uint `json:"roleId" binding:"required"`
}

type RoleResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func toRoleResponse(r Role) RoleResponse {
	return RoleResponse{ID: r.ID, Name: r.Name}
}

// GrantResourceAccessRequest doubles as both "grant" and "update" —
// granting again for the same (granteeType, granteeId, resourceType,
// resourceId) upserts the level/effect rather than erroring or
// duplicating, so there's no separate update request shape.
type GrantResourceAccessRequest struct {
	GranteeType  GranteeType    `json:"granteeType" binding:"required,oneof=user role"`
	GranteeID    uint           `json:"granteeId" binding:"required"`
	ResourceType string         `json:"resourceType" binding:"required,max=50"`
	ResourceID   uint           `json:"resourceId"`
	Level        AccessLevel    `json:"level" binding:"required,oneof=read write manage"`
	Effect       ResourceEffect `json:"effect" binding:"required,oneof=accepted prohibited"`
}

type ResourceAccessResponse struct {
	ID           uint           `json:"id"`
	GranteeType  GranteeType    `json:"granteeType"`
	GranteeID    uint           `json:"granteeId"`
	ResourceType string         `json:"resourceType"`
	ResourceID   uint           `json:"resourceId"`
	Level        AccessLevel    `json:"level"`
	Effect       ResourceEffect `json:"effect"`
}

func toResourceAccessResponse(a ResourceAccess) ResourceAccessResponse {
	return ResourceAccessResponse{
		ID:           a.ID,
		GranteeType:  a.GranteeType,
		GranteeID:    a.GranteeID,
		ResourceType: a.ResourceType,
		ResourceID:   a.ResourceID,
		Level:        a.Level,
		Effect:       a.Effect,
	}
}

// MyAccessResponse is the caller's own complete access profile — sent
// right after login so the frontend can decide what nav items and
// records to show without trial-and-error against 403s. ResourceAccess is
// every grant that's personally the caller's (direct or via a role they
// hold); Levels is the best level the caller holds on each known resource
// type at large (checked against shared.WildcardResourceID) — what drives
// nav-item and button visibility, since the frontend can't enumerate
// every resource ID up front.
type MyAccessResponse struct {
	ResourceAccess []ResourceAccessResponse `json:"resourceAccess"`
	Levels         map[string]AccessLevel   `json:"levels"`
}
