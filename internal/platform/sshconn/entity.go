package sshconn

import "github.com/Aliizi83/vohu/internal/platform/shared"

// AuthMethod is explicit rather than inferring from which secret field is
// set — a private key and a password are both just "a string" once
// encrypted, so the intent has to be recorded separately.
type AuthMethod string

const (
	AuthPassword   AuthMethod = "password"
	AuthPrivateKey AuthMethod = "private_key"
)

// SSHConnection is a database-defined target the agent can execute
// commands against over SSH. EncryptedSecret holds the password or
// private key, AES-GCM encrypted via pkg/crypto — never stored or
// returned in plaintext (see dto.go: Response never includes it in any
// form). Who may reach *this specific* connection is decided by
// rbac.ResourcePermission (resourceType "ssh_connection", resourceID =
// this row's ID), not by anything in this struct.
type SSHConnection struct {
	shared.BaseModel
	Name            string     `gorm:"type:varchar(100);not null"`
	Host            string     `gorm:"type:varchar(255);not null"`
	Port            int        `gorm:"not null;default:22"`
	Username        string     `gorm:"type:varchar(100);not null"`
	AuthMethod      AuthMethod `gorm:"type:varchar(20);not null"`
	EncryptedSecret string     `gorm:"type:text;not null"`
	CreatedByUserID uint       `gorm:"not null"`
}

func (SSHConnection) TableName() string { return "ssh_connections" }

func init() {
	shared.RegisterModel(&SSHConnection{})
}
