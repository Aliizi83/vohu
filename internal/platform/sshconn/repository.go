package sshconn

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

// Repository is plain CRUD — nothing about sshconn needs a hand-written
// lookup beyond what shared.GenericRepository already provides. Row
// visibility for a caller without the wildcard bypass is handled in the
// service layer (ListForCaller, via shared.FilterAndPaginate), not here.
type Repository = *shared.GenericRepository[SSHConnection]

func NewRepository(db *gorm.DB) Repository {
	return shared.NewGenericRepository[SSHConnection](db)
}
