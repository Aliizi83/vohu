package command

import (
	"context"
	"testing"
)

type fakeExecutor struct {
	lastCommand Command
	output      string
	err         error
}

func (f *fakeExecutor) Execute(ctx context.Context, cmd Command) (string, error) {
	f.lastCommand = cmd
	return f.output, f.err
}

func TestTool_Execute_MissingProgram(t *testing.T) {
	tool := NewTool(&fakeExecutor{})

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err == nil {
		t.Fatal("expected an error when program is missing")
	}
	if result.Success {
		t.Fatal("expected Success=false when program is missing")
	}
}

func TestTool_Execute_NonStringProgram(t *testing.T) {
	tool := NewTool(&fakeExecutor{})

	_, err := tool.Execute(context.Background(), map[string]any{"program": 123})
	if err == nil {
		t.Fatal("expected an error when program is not a string")
	}
}

func TestTool_Execute_ArgsMustBeArray(t *testing.T) {
	tool := NewTool(&fakeExecutor{})

	_, err := tool.Execute(context.Background(), map[string]any{
		"program": "ls",
		"args":    "not-an-array",
	})
	if err == nil {
		t.Fatal("expected an error when args is not an array")
	}
}

func TestTool_Execute_ArgsElementsMustBeStrings(t *testing.T) {
	tool := NewTool(&fakeExecutor{})

	_, err := tool.Execute(context.Background(), map[string]any{
		"program": "ls",
		"args":    []any{"ok", 123},
	})
	if err == nil {
		t.Fatal("expected an error when an args element is not a string")
	}
}

func TestTool_Execute_ValidCallDelegatesToExecutor(t *testing.T) {
	executor := &fakeExecutor{output: "total 0"}
	tool := NewTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{
		"program": "ls",
		"args":    []any{"-la"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got %+v", result)
	}
	if executor.lastCommand.Program != "ls" || len(executor.lastCommand.Args) != 1 || executor.lastCommand.Args[0] != "-la" {
		t.Fatalf("expected executor to receive Command{ls, [-la]}, got %+v", executor.lastCommand)
	}
}

func TestTool_Execute_ExecutorErrorIsReportedNotReturned(t *testing.T) {
	// Matches command.Tool's convention: an executor-level failure (command
	// denied by policy, exec failed, ...) is packed into a Success:false
	// ToolResult with a nil Go error — only invocation-shape problems
	// (missing/malformed args) return a real error.
	executor := &fakeExecutor{err: ErrCommandNotAllowed}
	tool := NewTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{"program": "rm"})
	if err != nil {
		t.Fatalf("expected nil error for an executor-level failure, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when the executor reports an error")
	}
}
