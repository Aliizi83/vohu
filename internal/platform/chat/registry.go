package chat

import (
	"context"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
)

// buildRegistry constructs one turn's tool registry from the DB: every
// agenttool.Tool row the caller is allowed to use (agentTools.
// ListForCaller — every public row, plus any private row they hold at
// least Read-level resource access to), mapped by name to its Go
// implementation (see builtinTool). A row whose Name doesn't match any
// case there (stale after a rename, say) is skipped rather than failing
// the whole turn — best-effort, same as ListSSHConnectionsTool skipping a
// row it can't confirm access to instead of erroring the whole list.
//
// Access to a tool *itself* (this function, via the row's own
// Visibility/grant) and access to the specific SSH *connection* a call
// names (checked again, every call, inside the tool's own Execute — see
// SSHTool and StubRemoteTool) are two independent checks on purpose:
// being allowed to use a tool at all doesn't imply being allowed to
// reach every connection that exists.
func buildRegistry(
	ctx context.Context,
	userID uint,
	agentTools agenttool.Service,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandRules commandrule.Service,
) (*tools.Registry, error) {
	rows, _, err := agentTools.ListForCaller(ctx, userID, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 500})
	if err != nil {
		return nil, err
	}

	registry := tools.NewRegistry()
	for _, row := range rows {
		if tool, ok := builtinTool(row.Name, userID, sshconns, canAccess, commandRules); ok {
			registry.Register(tool)
		}
	}

	return registry, nil
}

// builtinTool is every SSH-connection-bound tool the catalog can name —
// kept in sync with seeders.agentTools, which is what actually gives
// each of these a catalog row to be granted access to in the first
// place. ssh_execute and list_ssh_connections are real; everything else
// is a StubRemoteTool — see its doc comment for why, and agenttool's
// package doc comment for the bigger picture.
func builtinTool(
	name string,
	userID uint,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandRules commandrule.Service,
) (tools.Tool, bool) {
	switch name {
	case "list_ssh_connections":
		return NewListSSHConnectionsTool(userID, sshconns, canAccess), true
	case "ssh_execute":
		return NewSSHTool(userID, sshconns, canAccess, commandRules), true

	case "read_file":
		return NewStubRemoteTool(
			"read_file", "Read a file's contents.",
			map[string]ai_model.ToolProperty{
				"path": {Type: "string", Description: "Path to the file, on the remote host."},
			},
			[]string{"path"}, accessLevelRead, userID, sshconns, canAccess,
		), true
	case "write_file":
		return NewStubRemoteTool(
			"write_file", "Create or overwrite a file.",
			map[string]ai_model.ToolProperty{
				"path":    {Type: "string", Description: "Path to the file, on the remote host."},
				"content": {Type: "string", Description: "The full content to write."},
			},
			[]string{"path", "content"}, accessLevelWrite, userID, sshconns, canAccess,
		), true
	case "edit_file":
		return NewStubRemoteTool(
			"edit_file", "Replace one exact block of text in an existing file.",
			map[string]ai_model.ToolProperty{
				"path":    {Type: "string", Description: "Path to the file, on the remote host."},
				"old_str": {Type: "string", Description: "The exact text to replace. Must match exactly once."},
				"new_str": {Type: "string", Description: "The text to replace it with."},
			},
			[]string{"path", "old_str", "new_str"}, accessLevelWrite, userID, sshconns, canAccess,
		), true
	case "list_directory":
		return NewStubRemoteTool(
			"list_directory", "List files and directories at a path.",
			map[string]ai_model.ToolProperty{
				"path":      {Type: "string", Description: "Directory to list, on the remote host. Omit for the home directory."},
				"recursive": {Type: "boolean", Description: "List subdirectories' contents too."},
			},
			nil, accessLevelRead, userID, sshconns, canAccess,
		), true
	case "search_files":
		return NewStubRemoteTool(
			"search_files", "Search file contents by regex.",
			map[string]ai_model.ToolProperty{
				"pattern": {Type: "string", Description: "Regular expression to search for."},
				"path":    {Type: "string", Description: "File or directory to search, on the remote host."},
			},
			[]string{"pattern"}, accessLevelRead, userID, sshconns, canAccess,
		), true
	case "find_files":
		return NewStubRemoteTool(
			"find_files", "Find files by name/glob pattern.",
			map[string]ai_model.ToolProperty{
				"pattern": {Type: "string", Description: "Glob pattern to match, e.g. \"**/*.log\"."},
				"path":    {Type: "string", Description: "Directory to search under, on the remote host."},
			},
			[]string{"pattern"}, accessLevelRead, userID, sshconns, canAccess,
		), true

	default:
		return nil, false
	}
}
