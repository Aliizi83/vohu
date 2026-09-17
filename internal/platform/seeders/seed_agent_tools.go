package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"gorm.io/gorm"
)

// agentTools is every SSH-connection-bound tool chat.Handler currently
// knows how to construct — see chat/registry.go's builtinTool. The
// read_file/write_file/edit_file/list_directory/search_files/find_files
// stubs that used to live here are gone — see seed_custom_tools.go, which
// now owns real implementations of those names.
var agentTools = []struct {
	name        string
	description string
}{
	{"list_ssh_connections", "List the SSH connections the current user is permitted to use, with their IDs — call this before ssh_execute to find a valid connectionId."},
	{"ssh_execute", "Execute a command on a remote host over SSH, using an SSH connection already stored in the system."},
	{"create_custom_tool", "Author and register a brand-new tool from Go source code, for a job no existing tool covers."},
}

// seedAgentTools gives every SSH-connection-bound tool implementation a
// catalog row. Runs after seedRoles/seedAdminAccess only in the sense
// that admin's wildcard "manage" on resourceType "agent_tool" already
// covers these rows the moment they exist — no separate grant needed
// here.
func seedAgentTools(database *gorm.DB) error {
	for _, at := range agentTools {
		var existing agenttool.Tool

		err := database.Where("name = ?", at.name).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			tool := agenttool.Tool{
				Name:        at.name,
				Description: at.description,
				Visibility:  agenttool.VisibilityPublic,
				Implemented: true,
			}
			if err := database.Create(&tool).Error; err != nil {
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
