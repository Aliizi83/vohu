// Package customtool is the user/agent-authored tool catalog — unlike
// agenttool.Tool, rows here are created through this module's own API and
// their behavior comes from the Go source on their ToolVersion rows.
package customtool

import "github.com/Aliizi83/vohu/internal/platform/shared"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

const ResourceTypeCustomTool = "custom_tool"

type Tool struct {
	shared.BaseModel
	Name            string     `gorm:"type:varchar(100);not null;uniqueIndex"`
	Description     string     `gorm:"type:text;not null"`
	ParamsSchema    string     `gorm:"type:text;not null"`
	Visibility      Visibility `gorm:"type:varchar(20);not null;default:'private'"`
	CreatedByUserID uint       `gorm:"not null"`
}

func (Tool) TableName() string { return "custom_tools" }

// ToolVersion is immutable once created — a change is always a new
// version, never an edit, so a deployed binary always maps back to the
// source that produced it. Latest = highest ID for a Tool.
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
