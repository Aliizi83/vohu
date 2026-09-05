package rbac

type CreateRoleRequest struct {
	Name string `json:"name" binding:"required,min=2,max=50"`
}

type CreatePermissionRequest struct {
	Key string `json:"key" binding:"required,min=2,max=100"`
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
