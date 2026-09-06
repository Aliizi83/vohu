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

// AccessLevel is a graded grant rather than a bare accepted/forbidden
// bool, so "can user 7 read ssh_connection 3" and "can user 7 execute
// commands on ssh_connection 3" can have different answers instead of one
// all-or-nothing flag. Levels are ordered (see rank/Satisfies below);
// Forbidden is a hard floor that no requested level ever satisfies, even
// though numerically it sits below Read — it exists as an explicit,
// legible override in the database, not just "absence of a row" (a future
// wildcard/group grant could otherwise resurrect access a specific deny
// was meant to block).
type AccessLevel string

const (
	AccessForbidden AccessLevel = "forbidden"
	AccessRead      AccessLevel = "read"
	AccessWrite     AccessLevel = "write"
	AccessManage    AccessLevel = "manage"
)

var accessLevelRank = map[AccessLevel]int{
	AccessForbidden: -1,
	AccessRead:      1,
	AccessWrite:     2,
	AccessManage:    3,
}

// Satisfies reports whether this granted level meets or exceeds the
// required level — e.g. AccessManage.Satisfies(AccessRead) is true,
// AccessRead.Satisfies(AccessWrite) is false, and AccessForbidden never
// satisfies anything (its rank is below every real level, and the -1
// floor keeps it that way even if new levels are added below Read later).
func (level AccessLevel) Satisfies(required AccessLevel) bool {
	if level == AccessForbidden {
		return false
	}
	return accessLevelRank[level] >= accessLevelRank[required]
}

// ResourcePermission is the second, narrower permission mechanism next to
// Role/Permission above — it doesn't replace flat keys (user:create,
// rbac:manage, ...), it answers a different question: at what level can
// this specific user touch *this one row* of *this one resource type*
// (e.g. can user 7 write to ssh_connection 3). ResourceType is a free
// string, not a Go type reference — sshconn (or any future resource
// module) never has to be imported here, same decoupling rule every
// module already follows.
type ResourcePermission struct {
	shared.BaseModel
	UserID       uint
	ResourceType string `gorm:"type:varchar(50);not null"`
	ResourceID   uint
	Level        AccessLevel `gorm:"type:varchar(20);not null"`
}

func (ResourcePermission) TableName() string { return "resource_permissions" }

func init() {
	shared.RegisterModel(&Role{})
	shared.RegisterModel(&Permission{})
	shared.RegisterModel(&RolePermission{})
	shared.RegisterModel(&UserRole{})
	shared.RegisterModel(&ResourcePermission{})
}
