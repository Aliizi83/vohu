package tools

import (
	"context"
	"time"
)

const toolName = "get_current_system_time"
const toolDescription = "this is a first tool just for creating architecture, returns simple system date time"

type CurrentSystemTime struct{}

func (t CurrentSystemTime) Name() string {
	return toolName
}
func (t CurrentSystemTime) Description() string {
	return toolDescription
}
func (t CurrentSystemTime) Execute(ctx context.Context, args map[string]any) (any, error) {
	return time.Now(), nil
}
