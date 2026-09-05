package rbac

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Role and Permission carry no foreign-key struct fields to other modules'
// entities on purpose — a UserRole holds a plain UserID, not a *user.User —
// so rbac stays independent of the user module's internals.

type Role struct {
	shared.BaseModel
	Name string `gorm:"type:varchar(50);not null;unique"`
}

func (Role) TableName() string { return "roles" }

type Permission struct {
	shared.BaseModel
	Key string `gorm:"type:varchar(100);not null;unique"`
}

func (Permission) TableName() string { return "permissions" }

type RolePermission struct {
	shared.BaseModel
	RoleID       uint
	PermissionID uint
}

func (RolePermission) TableName() string { return "role_permissions" }

type UserRole struct {
	shared.BaseModel
	UserID uint
	RoleID uint
}

func (UserRole) TableName() string { return "user_roles" }

func init() {
	shared.RegisterModel(&Role{})
	shared.RegisterModel(&Permission{})
	shared.RegisterModel(&RolePermission{})
	shared.RegisterModel(&UserRole{})
}
