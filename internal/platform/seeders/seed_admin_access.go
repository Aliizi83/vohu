package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

// seedAdminAccess grants the admin role a wildcard "manage" ResourceAccess
// row for every known resource type — the single mechanism that bootstraps
// the whole authorization system. Since access is resolved per-resource
// (rbac.Service.HasAccessLevel), there's no "grant this role everything"
// shortcut left over from the old flat-permission system; a wildcard row
// (resourceID = shared.WildcardResourceID) is what makes "manage" apply to
// every existing *and future* row of that type, without needing a row per
// object. Runs after seedRoles.
func seedAdminAccess(database *gorm.DB) error {
	var adminRole rbac.Role
	if err := database.Where("name = ?", DefaultAdminRoleName).First(&adminRole).Error; err != nil {
		return err
	}

	for _, resourceType := range rbac.KnownResourceTypes {
		var existing rbac.ResourceAccess

		err := database.Where(
			"grantee_type = ? AND grantee_id = ? AND resource_type = ? AND resource_id = ?",
			rbac.GranteeRole, adminRole.ID, resourceType, shared.WildcardResourceID,
		).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			access := rbac.ResourceAccess{
				GranteeType:  rbac.GranteeRole,
				GranteeID:    adminRole.ID,
				ResourceType: resourceType,
				ResourceID:   shared.WildcardResourceID,
				Level:        rbac.AccessManage,
				Effect:       rbac.EffectAccepted,
			}
			if err := database.Create(&access).Error; err != nil {
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
