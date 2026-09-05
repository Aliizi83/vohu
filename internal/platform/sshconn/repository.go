package sshconn

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

// Repository is plain CRUD — nothing about sshconn needs a hand-written
// lookup beyond what shared.GenericRepository already provides, unlike
// user (FindByUsername) or rbac (FindResourcePermission).
type Repository = *shared.GenericRepository[SSHConnection]

func NewRepository(db *gorm.DB) Repository {
	return shared.NewGenericRepository[SSHConnection](db)
}
