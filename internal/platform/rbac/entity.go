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

// Effect is explicit rather than a bare bool so a row's intent is legible
// in the database itself, not just in Go: "forbidden" wins if a caller
// ever ends up with both an accepted and a forbidden row for the same
// (user, resource) pair.
type Effect string

const (
	EffectAccepted  Effect = "accepted"
	EffectForbidden Effect = "forbidden"
)

// ResourcePermission is the second, narrower permission mechanism next to
// Role/Permission above — it doesn't replace flat keys (user:create,
// rbac:manage, ...), it answers a different question: can this specific
// user touch *this one row* of *this one resource type* (e.g. can user 7
// reach ssh_connection 3). ResourceType is a free string, not a Go type
// reference — sshconn (or any future resource module) never has to be
// imported here, same decoupling rule every module already follows.
type ResourcePermission struct {
	shared.BaseModel
	UserID       uint
	ResourceType string `gorm:"type:varchar(50);not null"`
	ResourceID   uint
	Effect       Effect `gorm:"type:varchar(20);not null"`
}

func (ResourcePermission) TableName() string { return "resource_permissions" }

func init() {
	shared.RegisterModel(&Role{})
	shared.RegisterModel(&Permission{})
	shared.RegisterModel(&RolePermission{})
	shared.RegisterModel(&UserRole{})
	shared.RegisterModel(&ResourcePermission{})
}
