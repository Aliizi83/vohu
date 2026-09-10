// Package agenttool is the DB-backed catalog of tools the agent can call
// in the platform. Every tool in this catalog is SSH-connection-bound —
// its Go implementation (in internal/platform/chat) always takes a
// connectionId argument and checks the caller's access to that specific
// connection before doing anything, the same way ssh_execute already
// did. There is deliberately no "runs locally on the Vohu server" tool
// kind and no user-authored custom tool yet — see Tool's doc comment on
// Implemented for the other half of what's deliberately not built yet.
//
// Who may even see/use a given row is decided by rbac.ResourceAccess
// (resourceType ResourceTypeTool) for Private rows; Public rows skip
// that check entirely (see Service.ListForCaller). This is a second,
// independent layer from the per-connection check every tool's Execute
// still does on its own — being allowed to use read_file at all doesn't
// imply being allowed to reach every SSH connection that exists.
package agenttool

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Visibility: Public skips the access check in Service.ListForCaller
// entirely (everyone gets it); Private requires an explicit
// rbac.ResourceAccess grant (a role's wildcard "manage"/"read", or a
// specific per-row grant an admin sets up via the existing generic
// POST /resource-access — no new endpoint needed for that).
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

const ResourceTypeTool = "agent_tool"

// Tool is one entry in the catalog — every row here corresponds to a
// concrete Go tools.Tool implementation in internal/platform/chat (see
// chat/registry.go's builtinTool), matched by Name. There is no row a
// user creates through an API; the whole catalog is seeded at startup
// (seeders.seedAgentTools) from the fixed set of Go implementations that
// exist, and Update only ever changes Visibility.
type Tool struct {
	shared.BaseModel
	Name        string     `gorm:"type:varchar(100);not null;uniqueIndex"`
	Description string     `gorm:"type:text;not null"`
	Visibility  Visibility `gorm:"type:varchar(20);not null;default:'private'"`
	// Implemented is false for a tool whose Go Execute is a stub that
	// checks connection access and then reports "not implemented yet" —
	// every non-ssh_execute tool today, pending the lightweight remote
	// execution agent this whole catalog is structured to support once
	// it exists. Purely informational (the frontend uses it to show a
	// "not implemented" badge); it doesn't affect access control at all.
	Implemented bool `gorm:"not null;default:false"`
}

func (Tool) TableName() string { return "agent_tools" }

func init() {
	shared.RegisterModel(&Tool{})
}
