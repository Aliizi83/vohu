package commandrule

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/Aliizi83/vohu/internal/platform/shared"
)

// ArgsPrefixes is [][]string stored as one JSON-encoded text column —
// there's no array-of-arrays column type portable across Postgres and the
// sqlite used in tests, so it round-trips through Scan/Value instead of a
// native array/jsonb type.
type ArgsPrefixes [][]string

func (a ArgsPrefixes) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (a *ArgsPrefixes) Scan(value any) error {
	if value == nil {
		*a = nil
		return nil
	}

	var raw []byte
	switch v := value.(type) {
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("commandrule: unsupported type %T for ArgsPrefixes", value)
	}

	if len(raw) == 0 {
		*a = nil
		return nil
	}
	return json.Unmarshal(raw, a)
}

// Rule is one entry of a specific SSH connection's own command allow-list
// — a program name, optionally scoped to one or more argument prefixes
// (e.g. "git" + [["status"]] matches "git status ..." but not "git
// push"), and whether that match is allowed. Evaluated first-match-wins,
// exactly like internal/tools/command.Rule (chat.buildCommandPolicy
// converts a connection's rules into that type at execute time — this
// package never imports internal/tools, same decoupling rule every other
// platform module follows). A connection with zero rules allows nothing —
// there's no implicit default policy.
//
// Who may read/change a connection's rules is decided entirely by
// rbac.ResourceAccess on the connection itself (resourceType
// "ssh_connection", level "manage") — the same level sshconn.RegisterRoutes
// already gates deleting a connection on, since a connection's allowed
// commands are exactly as sensitive as the connection existing at all.
// There's no separate resource type or grant just for rules.
type Rule struct {
	shared.BaseModel
	SSHConnectionID uint         `gorm:"not null;index"`
	Program         string       `gorm:"type:varchar(255);not null"`
	ArgsPrefixes    ArgsPrefixes `gorm:"type:text"`
	// Allowed has no gorm "default" tag on purpose — gorm silently omits a
	// bool field from the INSERT when it's the Go zero value (false) and
	// tagged with a column default, letting the DB's default:true win over
	// an explicit false every time. Defaulting to true on an omitted
	// request field is handled once, in Service.Create, instead.
	Allowed bool `gorm:"not null"`
}

func (Rule) TableName() string { return "ssh_command_rules" }

func init() {
	shared.RegisterModel(&Rule{})
}
