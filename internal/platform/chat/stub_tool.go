package chat

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
)

// StubRemoteTool is every SSH-connection-bound tool that doesn't have a
// real remote execution mechanism yet — read_file, write_file,
// list_directory, and the rest, pending the lightweight agent binary
// this whole catalog is structured around (see agenttool's package doc
// comment). It still does the one thing that has to be real regardless:
// checking the caller's access to the specific connection they asked
// for, exactly like SSHTool.Execute does — a tool being "not implemented
// yet" is not a reason to skip access control, and it means no
// information about a connection the caller can't reach leaks through
// this stub either. What it can't do yet is anything past that check;
// Execute always reports unimplemented once access is confirmed.
type StubRemoteTool struct {
	name          string
	description   string
	extraParams   map[string]ai_model.ToolProperty
	requiredExtra []string
	requiredLevel string // "read" or "write", same meaning as SSHTool's

	userID    uint
	sshconns  sshconn.Service
	canAccess shared.AccessLevelCheck
}

func NewStubRemoteTool(
	name string,
	description string,
	extraParams map[string]ai_model.ToolProperty,
	requiredExtra []string,
	requiredLevel string,
	userID uint,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
) *StubRemoteTool {
	return &StubRemoteTool{
		name:          name,
		description:   description,
		extraParams:   extraParams,
		requiredExtra: requiredExtra,
		requiredLevel: requiredLevel,
		userID:        userID,
		sshconns:      sshconns,
		canAccess:     canAccess,
	}
}

func (t *StubRemoteTool) Name() string { return t.name }

func (t *StubRemoteTool) Description() string {
	return t.description + " (not implemented yet — no remote execution agent runs on the target host)"
}

func (t *StubRemoteTool) Parameters() ai_model.ToolParameters {
	props := map[string]ai_model.ToolProperty{
		"connectionId": {
			Type:        "integer",
			Description: "The ID of the SSH connection to run this against.",
		},
	}
	for k, v := range t.extraParams {
		props[k] = v
	}

	required := make([]string, 0, len(t.requiredExtra)+1)
	required = append(required, "connectionId")
	required = append(required, t.requiredExtra...)

	return ai_model.ToolParameters{Properties: props, Required: required}
}

func (t *StubRemoteTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	connectionID, ok := parseUintArg(args["connectionId"])
	if !ok {
		return tools.ToolResult{Success: false, Data: "connectionId is required and must be a number"}, nil
	}

	allowed, err := t.canAccess(ctx, t.userID, sshconn.ResourceTypeSSHConnection, connectionID, t.requiredLevel)
	if err != nil {
		return tools.ToolResult{Success: false, Data: fmt.Sprintf("permission check failed: %v", err)}, nil
	}
	if !allowed {
		return tools.ToolResult{Success: false, Data: "access to this SSH connection is not permitted"}, nil
	}

	return tools.ToolResult{
		Success: false,
		Data:    fmt.Sprintf("%s is not implemented yet — no remote execution agent is running on the target host", t.name),
	}, nil
}
