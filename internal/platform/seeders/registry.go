package seeders

import "gorm.io/gorm"

// SeederFunc and Seeders mirror sample-golang-project's
// infra/presistence/seeders/registry.go exactly. Order matters: role and
// permission rows must exist before the rows linking them do.
type SeederFunc func(database *gorm.DB) error

var Seeders = []SeederFunc{
	seedRoles,
	seedUsers,
	seedUserRoles,
	seedAdminAccess,
}
