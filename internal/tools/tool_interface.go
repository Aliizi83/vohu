package tools

import (
	"context"

	"github.com/Aliizi83/vohu/internal/ai_model"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() ai_model.ToolParameters
	Execute(ctx context.Context, args map[string]any) (ToolResult, error)
}

type ToolResult struct {
	Data    any
	Success bool
}
