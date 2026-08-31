package system_tools

import (
	"context"
	"time"

	"github.com/Aliizi83/vohu/internal/tools"
)

const toolName = "get_current_system_time"
const toolDescription = "this is a first tool just for creating architecture, returns simple system date time"

type CurrentSystemTime struct{}

func NewCurrentSystemTime() *CurrentSystemTime {
	return &CurrentSystemTime{}
}

func (t *CurrentSystemTime) Name() string {
	return toolName
}

func (t *CurrentSystemTime) Description() string {
	return toolDescription
}

func (t *CurrentSystemTime) Execute(
	ctx context.Context,
	args map[string]any,
) (tools.ToolResult, error) {

	now := time.Now()

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"time": now.Format(time.RFC3339),
		},
	}, nil
}
