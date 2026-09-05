package migrations

import (
	"github.com/Aliizi83/vohu/internal/platform/seeders"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/logging"
	"gorm.io/gorm"
)

// UpP_1 is the platform base's first migration: create every registered
// table (via shared.AllModels, populated by each entity's own init()),
// then seed default data. Numbered to match sample-golang-project's
// migration naming — a UpP_2 for the next schema change slots in the same
// way, called alongside this one from the composition root.
func UpP_1(database *gorm.DB, logger logging.Logger) error {
	if err := database.AutoMigrate(shared.AllModels...); err != nil {
		return err
	}
	logger.Info(logging.Postgres, logging.Migration, "tables migrated successfully", nil)

	for _, seed := range seeders.Seeders {
		if err := seed(database); err != nil {
			return err
		}
	}
	logger.Info(logging.Postgres, logging.Migration, "default data seeded", nil)

	return nil
}
