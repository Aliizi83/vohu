package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"gorm.io/gorm"
)

const DefaultAdminRoleName = "admin"

var defaultRoles = []string{DefaultAdminRoleName}

func seedRoles(database *gorm.DB) error {
	for _, name := range defaultRoles {
		var existing rbac.Role

		err := database.Where("name = ?", name).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := database.Create(&rbac.Role{Name: name}).Error; err != nil {
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
