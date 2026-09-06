package rbac

type CreateRoleRequest struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

type UpdateRoleRequest struct {
	Name string `json:"name" binding:"omitempty,min=2,max=50"`
}

type CreatePermissionRequest struct {
	Key string `json:"key" binding:"required,min=2,max=100"`
}

type UpdatePermissionRequest struct {
	Key string `json:"key" binding:"omitempty,min=2,max=100"`
}

type GrantPermissionRequest struct {
	PermissionID uint `json:"permissionId" binding:"required"`
}

type AssignRoleRequest struct {
	RoleID uint `json:"roleId" binding:"required"`
}

type RoleResponse struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type PermissionResponse struct {
	ID  uint   `json:"id"`
	Key string `json:"key"`
}

func toRoleResponse(r Role) RoleResponse {
	return RoleResponse{ID: r.ID, Name: r.Name}
}

func toPermissionResponse(p Permission) PermissionResponse {
	return PermissionResponse{ID: p.ID, Key: p.Key}
}

// GrantResourceAccessRequest doubles as both "grant" and "update" —
// granting again for the same (userId, resourceType, resourceId) upserts
// the level rather than erroring or duplicating, so there's no separate
// update request shape.
type GrantResourceAccessRequest struct {
	UserID       uint        `json:"userId" binding:"required"`
	ResourceType string      `json:"resourceType" binding:"required,max=50"`
	ResourceID   uint        `json:"resourceId" binding:"required"`
	Level        AccessLevel `json:"level" binding:"required,oneof=forbidden read write manage"`
}

type ResourcePermissionResponse struct {
	ID           uint        `json:"id"`
	UserID       uint        `json:"userId"`
	ResourceType string      `json:"resourceType"`
	ResourceID   uint        `json:"resourceId"`
	Level        AccessLevel `json:"level"`
}

func toResourcePermissionResponse(p ResourcePermission) ResourcePermissionResponse {
	return ResourcePermissionResponse{
		ID:           p.ID,
		UserID:       p.UserID,
		ResourceType: p.ResourceType,
		ResourceID:   p.ResourceID,
		Level:        p.Level,
	}
}

// MyAccessResponse is the caller's own complete access profile — sent
// right after login so the frontend can decide what nav items and
// records to show without trial-and-error against 403s. Permissions is
// every flat key the caller holds through any role; ResourceAccess is
// every per-resource grant they personally have (not everyone's, unlike
// the admin-only ListResourcePermissions).
type MyAccessResponse struct {
	Permissions    []string                     `json:"permissions"`
	ResourceAccess []ResourcePermissionResponse `json:"resourceAccess"`
}
