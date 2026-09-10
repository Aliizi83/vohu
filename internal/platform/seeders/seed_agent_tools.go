package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"gorm.io/gorm"
)

// agentTools is every SSH-connection-bound tool chat.Handler currently
// knows how to construct — see chat/registry.go's builtinTool. Adding a
// new one there also means adding it here, or it'll never appear in
// anyone's agenttool.Service.ListForCaller and so never get registered
// into a conversation no matter what access they're granted.
//
// All Public by default (unchanged behavior for ssh_execute/
// list_ssh_connections — every authenticated user already got those
// unconditionally before this catalog existed; the unimplemented ones
// are harmless to expose too since their Execute never does anything
// beyond the same connection-access check ssh_execute already makes).
// implemented mirrors chat/registry.go exactly: only the two tools with
// a real Go implementation past the access check are true.
var agentTools = []struct {
	name        string
	description string
	implemented bool
}{
	{"list_ssh_connections", "List the SSH connections the current user is permitted to use, with their IDs — call this before ssh_execute to find a valid connectionId.", true},
	{"ssh_execute", "Execute a command on a remote host over SSH, using an SSH connection already stored in the system.", true},

	{"read_file", "Read a file's contents.", false},
	{"write_file", "Create or overwrite a file.", false},
	{"edit_file", "Replace one exact block of text in an existing file.", false},
	{"list_directory", "List files and directories at a path.", false},
	{"search_files", "Search file contents by regex.", false},
	{"find_files", "Find files by name/glob pattern.", false},
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
				Implemented: at.implemented,
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
