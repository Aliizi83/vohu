package agenttool

import (
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

// Repository is plain CRUD via the generic repository — nothing here
// needs a hand-written lookup: rows are only ever written by the seeder
// (direct GORM, not through this interface) or Service.Update.
type Repository = *shared.GenericRepository[Tool]

func NewRepository(db *gorm.DB) Repository {
	return shared.NewGenericRepository[Tool](db)
}
