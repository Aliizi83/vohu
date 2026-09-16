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

// UpP_2 backfills conversations.archived for rows that predate that
// column — AutoMigrate adds a new column to an existing table as NULL
// regardless of the entity's own "not null; default:false" tag (those only
// govern new rows), and NULL matches neither `archived = false` nor
// `archived = true`, which would otherwise make every pre-existing
// conversation invisible in both the active and archived views. Idempotent
// — a no-op once no NULL rows remain.
func UpP_2(database *gorm.DB, logger logging.Logger) error {
	if err := database.Exec("UPDATE conversations SET archived = false WHERE archived IS NULL").Error; err != nil {
		return err
	}
	logger.Info(logging.Postgres, logging.Migration, "backfilled conversations.archived", nil)

	return nil
}
