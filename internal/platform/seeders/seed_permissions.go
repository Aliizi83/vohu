package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"gorm.io/gorm"
)

// KnownPermissions is every permission key the platform currently defines.
// New modules add their keys here as they're built, and the default admin
// role is granted all of them.
var KnownPermissions = []string{
	"user:create",
	"user:read",
	"rbac:manage",
}

func seedPermissions(database *gorm.DB) error {
	for _, key := range KnownPermissions {
		var existing rbac.Permission

		err := database.Where("key = ?", key).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.Create(&rbac.Permission{Key: key}).Error; err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
	}

	return nil
}
