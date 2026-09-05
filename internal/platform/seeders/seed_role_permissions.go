package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"gorm.io/gorm"
)

// seedRolePermissions grants the admin role every known permission. Runs
// after seedRoles and seedPermissions.
func seedRolePermissions(database *gorm.DB) error {
	var adminRole rbac.Role
	if err := database.Where("name = ?", DefaultAdminRoleName).First(&adminRole).Error; err != nil {
		return err
	}

	var permissions []rbac.Permission
	if err := database.Find(&permissions).Error; err != nil {
		return err
	}

	for _, permission := range permissions {
		var existing rbac.RolePermission

		err := database.Where("role_id = ? AND permission_id = ?", adminRole.ID, permission.ID).
			First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			rolePermission := rbac.RolePermission{RoleID: adminRole.ID, PermissionID: permission.ID}
			if err := database.Create(&rolePermission).Error; err != nil {
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
