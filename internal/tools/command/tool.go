package command

import (
	"context"
	"fmt"

	"github.com/Aliizi83/vohu/internal/tools"
)

type Tool struct {
	executor Executor
}

func NewTool(executor Executor) *Tool {
	return &Tool{
		executor: executor,
	}
}

func (t *Tool) Name() string {
	return "execute_command"
}

func (t *Tool) Description() string {
	return "Execute an operating system command."
}

func (t *Tool) Execute(
	ctx context.Context,
	args map[string]any,
) (tools.ToolResult, error) {

	program, ok := args["program"].(string)

	if !ok || program == "" {
		return tools.ToolResult{
			Success: false,
			Data:    "program is required",
		}, fmt.Errorf("program is required")
	}

	commandArgs := make([]string, 0)

	if rawArgs, exists := args["args"]; exists {

		argsList, ok := rawArgs.([]any)

		if !ok {
			return tools.ToolResult{
				Success: false,
				Data:    "args must be an array",
			}, fmt.Errorf("args must be an array")
		}

		for _, rawArg := range argsList {

			arg, ok := rawArg.(string)

			if !ok {
				return tools.ToolResult{
						Success: false,
						Data:    "all command arguments must be strings",
					}, fmt.Errorf(
						"command argument must be a string",
					)
			}

			commandArgs = append(
				commandArgs,
				arg,
			)
		}
	}

	cmd := Command{
		Program: program,
		Args:    commandArgs,
	}

	output, err := t.executor.Execute(
		ctx,
		cmd,
	)

	if err != nil {
		return tools.ToolResult{
			Success: false,
			Data: map[string]any{
				"output": output,
				"error":  err.Error(),
			},
		}, nil
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"output": output,
		},
	}, nil
}
