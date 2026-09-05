package command

import (
	"context"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func TestShellQuoteCommand_EscapesEmbeddedQuotes(t *testing.T) {
	cmd := Command{Program: "echo", Args: []string{"it's a test"}}

	got := shellQuoteCommand(cmd)
	want := `'echo' 'it'\''s a test'`
	if got != want {
		t.Fatalf("shellQuoteCommand() = %q, want %q", got, want)
	}
}

func TestShellQuoteCommand_PreventsInjectionViaShellMetacharacters(t *testing.T) {
	// A naive unquoted join would let this arg terminate the command and
	// run a second one; single-quoting must keep it inert as one literal
	// argument.
	cmd := Command{Program: "echo", Args: []string{"hi; rm -rf /"}}

	got := shellQuoteCommand(cmd)
	want := `'echo' 'hi; rm -rf /'`
	if got != want {
		t.Fatalf("shellQuoteCommand() = %q, want %q", got, want)
	}
}

func TestShellQuoteCommand_NoArgs(t *testing.T) {
	cmd := Command{Program: "pwd"}

	got := shellQuoteCommand(cmd)
	want := `'pwd'`
	if got != want {
		t.Fatalf("shellQuoteCommand() = %q, want %q", got, want)
	}
}

// TestSSHExecutor_Execute_DeniedCommandNeverDials confirms the policy
// check happens before any network I/O: pointing at a non-routable
// address (RFC 5737 TEST-NET-1) that would hang until dial's own timeout
// if actually attempted, then asserting a denied command returns well
// under that timeout is how we know Execute short-circuited on the
// policy, not on a fast connection failure.
func TestSSHExecutor_Execute_DeniedCommandNeverDials(t *testing.T) {
	policy := NewCommandPolicy(PolicyModeAccept, nil) // accept mode + no rules == everything denied

	executor := NewSSHExecutor("203.0.113.1", 22, "user", ssh.Password("unused"), policy)

	start := time.Now()
	output, err := executor.Execute(context.Background(), Command{Program: "ls"})
	elapsed := time.Since(start)

	if err != ErrCommandNotAllowed {
		t.Fatalf("expected ErrCommandNotAllowed, got %v", err)
	}
	if output == "" {
		t.Fatal("expected a non-empty policy denial reason")
	}
	if elapsed > 2*time.Second {
		t.Fatalf("Execute took %v — looks like it attempted a network dial instead of short-circuiting on policy", elapsed)
	}
}
