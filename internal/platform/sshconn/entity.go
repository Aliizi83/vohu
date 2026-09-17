package sshconn

import "github.com/Aliizi83/vohu/internal/platform/shared"

// SSHConnection is a database-defined target the agent can execute
// commands against over SSH — private-key auth only, no password option
// (a live TestConnectionFunc check confirms the key actually works before
// Service.Create ever writes a row, catching a typo'd host or a mismatched
// key immediately rather than the first time the agent tries to use it).
// EncryptedPrivateKey is AES-GCM encrypted via pkg/crypto — never stored
// or returned in plaintext (see dto.go: Response never includes it in any
// form). Who may reach *this specific* connection is decided by
// rbac.ResourceAccess (resourceType "ssh_connection", resourceID = this
// row's ID), not by anything in this struct.
// CommandPolicyMode values mirror command.PolicyMode's own string values —
// this package never imports internal/tools/command directly (same
// decoupling rule commandrule/ssh_tool.go's buildCommandPolicy doc comment
// already documents), so they're just plain strings here, translated back
// to the real type at the one call site that builds a command.Policy.
const (
	// CommandPolicyModeAccept is the safe-by-default mode: a command not
	// matched by any of this connection's rules is denied. This is what
	// every connection had, unconditionally, before the mode became a
	// per-connection, toggleable setting.
	CommandPolicyModeAccept = "accept"
	// CommandPolicyModeProhibited inverts that: every command is allowed
	// except one matched by a rule — "deny only what's listed" instead of
	// "allow only what's listed".
	CommandPolicyModeProhibited = "prohibited"
)

type SSHConnection struct {
	shared.BaseModel
	Name                string `gorm:"type:varchar(100);not null"`
	Host                string `gorm:"type:varchar(255);not null"`
	Port                int    `gorm:"not null;default:22"`
	Username            string `gorm:"type:varchar(100);not null"`
	EncryptedPrivateKey string `gorm:"type:text;not null"`
	// CommandPolicyMode governs how this connection's commandrule.Rule
	// rows are interpreted for ssh_execute — see the two constants above.
	// AutoMigrate adds a new column as NULL for pre-existing rows
	// regardless of this "default" tag (it only governs new rows), so
	// migrations.UpP_4 backfills it for rows that predate this field.
	CommandPolicyMode string `gorm:"type:varchar(20);not null;default:'accept'"`
	CreatedByUserID   uint   `gorm:"not null"`
}

func (SSHConnection) TableName() string { return "ssh_connections" }

func init() {
	shared.RegisterModel(&SSHConnection{})
}
