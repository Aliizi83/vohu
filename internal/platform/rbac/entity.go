package rbac

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Role is a named group a user can belong to (via UserRole) — nothing
// more. It carries no flat permission keys of its own; a role's actual
// capabilities come entirely from ResourceAccess rows granted to it (see
// below). It carries no foreign-key struct field to user.User either —
// same decoupling rule every module follows.
type Role struct {
	shared.BaseModel
	Name string `gorm:"type:varchar(50);not null;unique"`
}

func (Role) TableName() string { return "roles" }

type UserRole struct {
	shared.BaseModel
	UserID uint
	RoleID uint
}

func (UserRole) TableName() string { return "user_roles" }

// GranteeType is who a ResourceAccess row's grant belongs to — a specific
// user, or every member of a role at once.
type GranteeType string

const (
	GranteeUser GranteeType = "user"
	GranteeRole GranteeType = "role"
)

// ResourceEffect is explicit rather than folded into AccessLevel so a
// row's intent is legible in the database itself: "prohibited" is a hard
// veto (see AccessLevel.Satisfies), distinct from simply granting a low
// level.
type ResourceEffect string

const (
	EffectAccepted   ResourceEffect = "accepted"
	EffectProhibited ResourceEffect = "prohibited"
)

// AccessLevel is a graded grant — "can read", "can write", "can manage" —
// rather than a bare yes/no, so "can user A write to resource B" and "can
// user A merely read resource B" can have different answers.
type AccessLevel string

const (
	AccessRead   AccessLevel = "read"
	AccessWrite  AccessLevel = "write"
	AccessManage AccessLevel = "manage"
)

var accessLevelRank = map[AccessLevel]int{
	AccessRead:   1,
	AccessWrite:  2,
	AccessManage: 3,
}

// Satisfies reports whether this granted level meets or exceeds the
// required level — e.g. AccessManage.Satisfies(AccessRead) is true,
// AccessRead.Satisfies(AccessWrite) is false. Whether a row applies at
// all (Effect == accepted) is a separate question the resolver checks
// first — this only compares levels once a row's Effect already qualifies
// it.
func (level AccessLevel) Satisfies(required AccessLevel) bool {
	return accessLevelRank[level] >= accessLevelRank[required]
}

// ResourceAccess is the platform's one authorization mechanism: at what
// level (if any) can a grantee (a specific user, or every member of a
// role) touch a specific resource (or every resource of a type, via
// shared.WildcardResourceID). ResourceType is a free string, not a Go
// type reference — sshconn (or any other resource module, "user", "role",
// "conversation", "provider_key", ...) never has to be imported here,
// same decoupling rule every module already follows.
//
// Granting access to a Role (ResourceType "role") cascades to every user
// who holds that role — see Service.HasAccessLevel — which is what lets a
// single grant say "this role can read every regular user's
// conversations." A more specific row directly on one user (Effect
// prohibited) carves that one user back out of the cascade; the resolver
// always checks the most specific applicable row first.
type ResourceAccess struct {
	shared.BaseModel
	GranteeType  GranteeType
	GranteeID    uint
	ResourceType string `gorm:"type:varchar(50);not null"`
	ResourceID   uint
	Level        AccessLevel    `gorm:"type:varchar(20);not null"`
	Effect       ResourceEffect `gorm:"type:varchar(20);not null"`
}

func (ResourceAccess) TableName() string { return "resource_access" }

func init() {
	shared.RegisterModel(&Role{})
	shared.RegisterModel(&UserRole{})
	shared.RegisterModel(&ResourceAccess{})
}
