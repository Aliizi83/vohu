package command

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestShellTool_Execute_MissingCommand(t *testing.T) {
	tool := NewShellTool(&fakeExecutor{})

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when command is missing")
	}
}

func TestShellTool_Execute_RunsThroughAShell(t *testing.T) {
	executor := &fakeExecutor{output: "ok"}
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "ls -la | grep foo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got %+v", result.Data)
	}

	// The whole pipeline is handed to "sh -c" as one string — Program
	// alone never sees "ls", which is exactly why Policy is evaluated
	// against shell execution as a unit, not per-inner-command (see
	// ShellTool's doc comment).
	if executor.lastCommand.Program != "sh" {
		t.Fatalf("expected Program=sh, got %q", executor.lastCommand.Program)
	}
	if len(executor.lastCommand.Args) != 2 || executor.lastCommand.Args[0] != "-c" || executor.lastCommand.Args[1] != "ls -la | grep foo" {
		t.Fatalf("expected Args=[-c, <command>], got %+v", executor.lastCommand.Args)
	}
}

func TestShellTool_Execute_PassesWorkingDir(t *testing.T) {
	executor := &fakeExecutor{}
	tool := NewShellTool(executor)

	if _, err := tool.Execute(context.Background(), map[string]any{
		"command": "pwd", "working_dir": "/tmp",
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executor.lastCommand.Dir != "/tmp" {
		t.Fatalf("expected Dir=/tmp, got %q", executor.lastCommand.Dir)
	}
}

func TestShellTool_Execute_PolicyDenialIsReportedNotReturned(t *testing.T) {
	executor := &fakeExecutor{err: ErrCommandNotAllowed}
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{"command": "rm -rf /"})
	if err != nil {
		t.Fatalf("expected nil error for a policy denial, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when the policy denies shell execution")
	}
}

func TestShellTool_Execute_InvalidTimeoutIsRejected(t *testing.T) {
	tool := NewShellTool(&fakeExecutor{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hi", "timeout_seconds": float64(-5),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false for a non-positive timeout")
	}
}

func TestShellTool_Execute_TimeoutIsCappedNotRejected(t *testing.T) {
	// A caller-requested timeout above maxShellTimeout is clamped, not an
	// error — the context passed to the executor should reflect the cap.
	executor := &blockingExecutor{unblock: make(chan struct{})}
	close(executor.unblock) // don't actually block this test

	tool := NewShellTool(executor)
	_, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo hi", "timeout_seconds": float64(999999),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if executor.gotTimeout > maxShellTimeout {
		t.Fatalf("expected the context deadline to be capped at %s, effective timeout was %s", maxShellTimeout, executor.gotTimeout)
	}
}

func TestShellTool_Execute_OutputIsTruncated(t *testing.T) {
	huge := make([]byte, maxShellOutputChars*2)
	for i := range huge {
		huge[i] = 'x'
	}
	executor := &fakeExecutor{output: string(huge)}
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{"command": "yes x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := result.Data.(map[string]any)
	output := data["output"].(string)
	if len(output) >= len(huge) {
		t.Fatalf("expected output to be truncated, got length %d (original %d)", len(output), len(huge))
	}
}

// TestShellTool_Execute_NonZeroExitIsNotAToolFailure is a regression test
// for a real bug: a command that runs to completion and just returns a
// non-zero exit code (grep finding no match, a failed assertion, ...) was
// being reported as Success:false — identical to a policy denial or a
// genuine crash — losing the distinction between "the tool couldn't run
// this" and "the tool ran this and here's what happened." Uses a real
// LocalExecutor + real /bin/sh, not a fake, since the bug lived in how
// ShellTool classifies the concrete *exec.ExitError CombinedOutput
// returns for a non-zero exit — a fake executor can't reproduce that
// shape without just hand-waving the same assumption the bug was in.
func TestShellTool_Execute_NonZeroExitIsNotAToolFailure(t *testing.T) {
	executor := NewLocalExecutor(NewCommandPolicy(PolicyModeProhibited, nil))
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo some-output; exit 3",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true for a command that ran and simply exited non-zero, got Success=false with %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	if data["exitCode"] != 3 {
		t.Fatalf("expected exitCode=3, got %+v", data["exitCode"])
	}
	if data["output"] != "some-output\n" {
		t.Fatalf("expected output to still come through, got %q", data["output"])
	}
}

func TestShellTool_Execute_SuccessfulCommandReportsExitCodeZero(t *testing.T) {
	executor := NewLocalExecutor(NewCommandPolicy(PolicyModeProhibited, nil))
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{"command": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["exitCode"] != 0 {
		t.Fatalf("expected exitCode=0, got %+v", data["exitCode"])
	}
}

func TestShellTool_Execute_GenuineInfraFailureStaysAFailure(t *testing.T) {
	// A non-ExitError failure (couldn't even start the process, e.g. a
	// missing binary) must still be reported as Success:false — the fix
	// only reclassifies a completed process's own exit code, nothing else.
	executor := &fakeExecutor{err: errors.New("exec: \"sh\": executable file not found in $PATH")}
	tool := NewShellTool(executor)

	result, err := tool.Execute(context.Background(), map[string]any{"command": "echo hi"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when the process never started at all")
	}
}

// blockingExecutor records the deadline actually present on the context
// it was called with, as a stand-in for asserting ShellTool applied (and
// capped) a timeout without needing a real slow command in the test.
type blockingExecutor struct {
	unblock    chan struct{}
	gotTimeout time.Duration
}

func (b *blockingExecutor) Execute(ctx context.Context, cmd Command) (string, error) {
	if deadline, ok := ctx.Deadline(); ok {
		b.gotTimeout = time.Until(deadline)
	}
	<-b.unblock
	return "", nil
}
