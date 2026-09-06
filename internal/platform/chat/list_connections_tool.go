package chat

import (
	"context"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
)

// accessLevelRead is what merely appearing in this discovery listing
// requires — lower than the Write level SSHTool needs to actually execute
// anything, so a user can be granted "you may see this connection exists"
// without also being able to use it.
const accessLevelRead = "read"

// ListSSHConnectionsTool lets the model discover which SSH connections
// exist and their IDs before calling SSHTool — without it, the model has
// no way to know a valid connectionId short of the user typing one into
// the chat. Only connections canAccess grants at least Read on are
// returned; the point of the whole resource-permission system is that a
// user shouldn't even learn a connection exists if they can't reach it at
// all.
type ListSSHConnectionsTool struct {
	userID    uint
	sshconns  sshconn.Service
	canAccess shared.AccessLevelCheck
}

func NewListSSHConnectionsTool(userID uint, sshconns sshconn.Service, canAccess shared.AccessLevelCheck) *ListSSHConnectionsTool {
	return &ListSSHConnectionsTool{userID: userID, sshconns: sshconns, canAccess: canAccess}
}

func (t *ListSSHConnectionsTool) Name() string { return "list_ssh_connections" }

func (t *ListSSHConnectionsTool) Description() string {
	return "List the SSH connections the current user is permitted to use, with their IDs — call this before ssh_execute to find a valid connectionId."
}

func (t *ListSSHConnectionsTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{}
}

func (t *ListSSHConnectionsTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	// A generous page size rather than real pagination — this tool is for
	// a model to skim a human-scale list, not to browse it.
	items, _, err := t.sshconns.List(ctx, shared.DynamicFilter{}, shared.Pagination{PageNumber: 1, PageSize: 200})
	if err != nil {
		return tools.ToolResult{Success: false, Data: "failed to list connections"}, nil
	}

	visible := make([]map[string]any, 0, len(items))
	for _, conn := range items {
		allowed, err := t.canAccess(ctx, t.userID, sshconn.ResourceTypeSSHConnection, conn.ID, accessLevelRead)
		if err != nil || !allowed {
			continue
		}

		visible = append(visible, map[string]any{
			"id":       conn.ID,
			"name":     conn.Name,
			"host":     conn.Host,
			"username": conn.Username,
		})
	}

	return tools.ToolResult{Success: true, Data: visible}, nil
}
