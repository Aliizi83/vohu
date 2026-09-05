package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/rbac"
	"gorm.io/gorm"
)

// KnownPermissions is every permission key the platform currently defines.
// New modules add their keys here as they're built, and the default admin
// role is granted all of them.
//
// Two families:
//   - REST resource permissions, "<resource>:<verb>" — one per enforced
//     route, verb in {create, read, update, delete, manage}. "manage" is
//     used for a module (like rbac) whose admin surface isn't yet split
//     into per-action permissions.
//   - Shell command permissions, "command:<program>" or
//     "command:<program>:<subcommand>" — mirroring Vohu's own
//     command.CommandPolicy allow-list (cmd/vohu/main.go: pwd, ls, whoami,
//     git status/log, docker ps/logs). Nothing in this base enforces these
//     yet — no command-execution route exists here — but the keys exist
//     and are manageable via the rbac API now, ready for when this base
//     merges with Vohu's agent core and its execute_command tool.
var KnownPermissions = []string{
	// user
	"user:create",
	"user:read",
	"user:update",
	"user:delete",

	// rbac
	"rbac:manage",

	// ssh connections (who may manage connection rows at all — actual
	// per-connection usage is gated separately by
	// rbac.ResourcePermission, granted automatically to a connection's
	// creator and manageable via the rbac API from there)
	"ssh:create",
	"ssh:read",
	"ssh:update",
	"ssh:delete",

	// shell commands (not yet enforced anywhere in this base)
	"command:pwd",
	"command:ls",
	"command:whoami",
	"command:git:status",
	"command:git:log",
	"command:docker:ps",
	"command:docker:logs",
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
