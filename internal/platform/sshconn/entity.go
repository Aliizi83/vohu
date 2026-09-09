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
type SSHConnection struct {
	shared.BaseModel
	Name                string `gorm:"type:varchar(100);not null"`
	Host                string `gorm:"type:varchar(255);not null"`
	Port                int    `gorm:"not null;default:22"`
	Username            string `gorm:"type:varchar(100);not null"`
	EncryptedPrivateKey string `gorm:"type:text;not null"`
	CreatedByUserID     uint   `gorm:"not null"`
}

func (SSHConnection) TableName() string { return "ssh_connections" }

func init() {
	shared.RegisterModel(&SSHConnection{})
}
