package tools

import "context"

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]any) (any, error)
}
