// Package customtool is the user/agent-authored tool catalog — unlike
// agenttool.Tool (a fixed set seeded from built-in Go implementations),
// every row here is created through this module's own API, and its
// behavior comes from the Go source stored on its ToolVersion rows, not a
// compiled-in Go type. See ToolVersion's doc comment for the build/deploy
// story this is step one of.
package customtool

import "github.com/Aliizi83/vohu/internal/platform/shared"

// Visibility mirrors agenttool.Visibility exactly (same public/private
// split, same reasoning) — duplicated rather than imported, same
// decoupling rule every platform module follows.
type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

const ResourceTypeCustomTool = "custom_tool"

// Tool is one user/agent-defined tool definition. Its actual executable
// behavior lives on its ToolVersion rows, not here — Tool only carries the
// facts that don't change per version: the stable Name the agent calls it
// by, its Description (for the model to decide when to use it), and its
// ParamsSchema (raw JSON Schema text describing its input parameters).
type Tool struct {
	shared.BaseModel
	Name            string     `gorm:"type:varchar(100);not null;uniqueIndex"`
	Description     string     `gorm:"type:text;not null"`
	ParamsSchema    string     `gorm:"type:text;not null"`
	Visibility      Visibility `gorm:"type:varchar(20);not null;default:'private'"`
	CreatedByUserID uint       `gorm:"not null"`
}

func (Tool) TableName() string { return "custom_tools" }

// ToolVersion is one immutable build of a Tool's source — created once,
// never edited; a change is always a new version, never a mutation of an
// old one, so a binary already deployed on some target host always maps
// back to exactly the source that produced it. Service.LatestVersion
// (whatever ToolVersion has the highest ID for a Tool) is what gets
// built, deployed, and run whenever the agent calls that tool by name —
// there's no per-call version pinning in this design.
//
// SourceCode is plain Go source (a full `package main`) — external module
// imports are allowed (this project's build step is expected to run `go
// build` with normal module resolution, not a stdlib-only sandbox).
type ToolVersion struct {
	shared.BaseModel
	ToolID          uint   `gorm:"not null;uniqueIndex:idx_customtool_version"`
	Version         string `gorm:"type:varchar(50);not null;uniqueIndex:idx_customtool_version"`
	SourceCode      string `gorm:"type:text;not null"`
	CreatedByUserID uint   `gorm:"not null"`
}

func (ToolVersion) TableName() string { return "custom_tool_versions" }

func init() {
	shared.RegisterModel(&Tool{})
	shared.RegisterModel(&ToolVersion{})
}
