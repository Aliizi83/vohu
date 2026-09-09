package command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

const (
	defaultShellTimeout = 120 * time.Second
	maxShellTimeout     = 600 * time.Second
	maxShellOutputChars = 30_000
)

// ShellTool runs a command through a real shell ("sh -c <command>"),
// unlike Tool (execute_command), which runs one program with no shell
// involved at all. That's a deliberate, narrower trust decision, not an
// oversight: once arbitrary shell syntax is on the table — pipes, `&&`,
// `;`, `$(...)`, backticks — there is no sound way to allow-list "which
// commands are OK inside the string" by inspecting it; a Policy rule
// keyed on individual program names would only ever see the first token
// and nothing chained after it. So Policy.Evaluate here is checked
// against the shell invocation as a single opaque unit (Program: "sh"),
// the same way any other single program is gated — a deployment decides
// once whether shell execution is allowed at all, rather than getting a
// false sense of granular safety it can't actually deliver once a shell
// is involved.
type ShellTool struct {
	executor Executor
}

func NewShellTool(executor Executor) *ShellTool {
	return &ShellTool{executor: executor}
}

func (t *ShellTool) Name() string { return "execute_shell" }

func (t *ShellTool) Description() string {
	return "Run a shell command (pipes, redirects, and other shell syntax all work, unlike execute_command). Runs through the same security policy as execute_command, evaluated against shell execution as a whole rather than the individual commands inside it."
}

func (t *ShellTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"command": {
				Type:        "string",
				Description: "The shell command to run, e.g. \"ls -la | grep foo\".",
			},
			"working_dir": {
				Type:        "string",
				Description: "Directory to run the command in. Omit to use the process's own working directory.",
			},
			"timeout_seconds": {
				Type:        "integer",
				Description: "Kill the command if it hasn't finished after this many seconds. Defaults to 120, capped at 600.",
			},
		},
		Required: []string{"command"},
	}
}

func (t *ShellTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	command, ok := args["command"].(string)
	if !ok || command == "" {
		return tools.ToolResult{Success: false, Data: `"command" is required`}, nil
	}

	workingDir, _ := args["working_dir"].(string)

	timeout := defaultShellTimeout
	if raw, exists := args["timeout_seconds"]; exists && raw != nil {
		// Tool-call arguments arrive as map[string]any decoded from JSON
		// by whichever provider SDK parsed the model's response — numbers
		// most commonly land as float64, but int/int64 aren't assumed away.
		var seconds float64
		switch v := raw.(type) {
		case float64:
			seconds = v
		case int:
			seconds = float64(v)
		case int64:
			seconds = float64(v)
		default:
			return tools.ToolResult{Success: false, Data: `"timeout_seconds" must be a number`}, nil
		}
		timeout = time.Duration(seconds) * time.Second
		if timeout <= 0 {
			return tools.ToolResult{Success: false, Data: `"timeout_seconds" must be positive`}, nil
		}
		if timeout > maxShellTimeout {
			timeout = maxShellTimeout
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	output, err := t.executor.Execute(runCtx, Command{
		Program: "sh",
		Args:    []string{"-c", command},
		Dir:     workingDir,
	})
	output = tools.Truncate(output, maxShellOutputChars)

	if err != nil {
		if errors.Is(err, ErrCommandNotAllowed) {
			return tools.ToolResult{
				Success: false,
				Data:    map[string]any{"output": output, "error": "shell execution is not allowed by policy"},
			}, nil
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
			return tools.ToolResult{
				Success: false,
				Data:    map[string]any{"output": output, "error": fmt.Sprintf("command timed out after %s", timeout)},
			}, nil
		}

		// The command actually ran and the shell just returned a non-zero
		// exit code — grep finding no match, a test asserting something
		// false, a linter reporting issues, ... all completely normal
		// outcomes, not a failure of the tool itself. Report it as data
		// (exitCode alongside output) with Success:true, the same way a
		// real terminal doesn't treat "$? != 0" as its own malfunction —
		// only genuine infra-level failures (policy denial above, timeout
		// above, or the process never starting at all — a missing "sh",
		// permission denied, ...) are Success:false.
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return tools.ToolResult{
				Success: true,
				Data:    map[string]any{"output": output, "exitCode": exitErr.ExitCode()},
			}, nil
		}

		return tools.ToolResult{
			Success: false,
			Data:    map[string]any{"output": output, "error": err.Error()},
		}, nil
	}

	return tools.ToolResult{
		Success: true,
		Data:    map[string]any{"output": output, "exitCode": 0},
	}, nil
}
