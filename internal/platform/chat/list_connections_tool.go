package chat

import (
	"context"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
)

// ListSSHConnectionsTool lets the model discover which SSH connections
// exist and their IDs before calling SSHTool — without it, the model has
// no way to know a valid connectionId short of the user typing one into
// the chat. Only connections CanAccessResource actually allows for this
// user are returned; the point of the whole resource-permission system is
// that a user shouldn't even learn a connection exists if they can't use
// it.
type ListSSHConnectionsTool struct {
	userID    uint
	sshconns  sshconn.Service
	canAccess CanAccessResource
}

func NewListSSHConnectionsTool(userID uint, sshconns sshconn.Service, canAccess CanAccessResource) *ListSSHConnectionsTool {
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
		allowed, err := t.canAccess(ctx, t.userID, sshconn.ResourceTypeSSHConnection, conn.ID)
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
