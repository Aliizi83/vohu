package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"gorm.io/gorm"
)

// seedUserRoles assigns the admin role to the default admin user. Runs
// after seedRoles and seedUsers.
func seedUserRoles(database *gorm.DB) error {
	var admin user.User
	if err := database.Where("username = ?", DefaultAdminUsername).First(&admin).Error; err != nil {
		return err
	}

	var adminRole rbac.Role
	if err := database.Where("name = ?", DefaultAdminRoleName).First(&adminRole).Error; err != nil {
		return err
	}

	var existing rbac.UserRole

	err := database.Where("user_id = ? AND role_id = ?", admin.ID, adminRole.ID).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return database.Create(&rbac.UserRole{UserID: admin.ID, RoleID: adminRole.ID}).Error
	}

	return err
}
