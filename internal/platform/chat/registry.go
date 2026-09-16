package chat

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tooldeploy"
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
// names (checked again, every call, inside the tool's own Execute) are
// two independent checks on purpose: being allowed to use a tool at all
// doesn't imply being allowed to reach every connection that exists.
func buildRegistry(
	ctx context.Context,
	userID uint,
	agentTools agenttool.Service,
	customTools customtool.Service,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandRules commandrule.Service,
	deployer *tooldeploy.Deployer,
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

	customRows, _, err := customTools.ListToolsForCaller(ctx, userID, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 500})
	if err != nil {
		return nil, err
	}
	for _, row := range customRows {
		registry.Register(NewCustomTool(row, userID, sshconns, canAccess, customTools, deployer))
	}

	return registry, nil
}

// builtinTool: read_file/write_file/edit_file/list_directory/search_files/
// find_files used to be StubRemoteTool placeholders here — they're gone
// now that customtool has real implementations for them, not yet wired
// into this registry (that's the next phase).
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
	default:
		return nil, false
	}
}
