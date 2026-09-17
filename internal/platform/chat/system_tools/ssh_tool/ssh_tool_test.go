package ssh_tool

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/chat/system_tools/testsupport"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools/command"
)

func TestBuildCommandPolicy_AcceptModeDeniesUnmatched(t *testing.T) {
	policy := buildCommandPolicy(sshconn.CommandPolicyModeAccept, nil)
	if policy.Evaluate(command.Command{Program: "ls"}).Allowed {
		t.Fatal("expected accept mode with no matching rule to deny the command")
	}
}

func TestBuildCommandPolicy_ProhibitedModeAllowsUnmatched(t *testing.T) {
	policy := buildCommandPolicy(sshconn.CommandPolicyModeProhibited, nil)
	if !policy.Evaluate(command.Command{Program: "ls"}).Allowed {
		t.Fatal("expected prohibited mode with no matching rule to allow the command")
	}
}

func TestBuildCommandPolicy_UnrecognizedModeFallsBackToAccept(t *testing.T) {
	policy := buildCommandPolicy("", nil)
	if policy.Evaluate(command.Command{Program: "ls"}).Allowed {
		t.Fatal("expected an empty/unrecognized mode to fall back to accept (deny unmatched), the safe direction")
	}
}

func TestSSHTool_Execute_MissingConnectionID(t *testing.T) {
	tool := NewSSHTool(1, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{"program": "ls"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when connectionId is missing")
	}
}

func TestSSHTool_Execute_MissingProgram(t *testing.T) {
	tool := NewSSHTool(1, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(5)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when program is missing")
	}
}

func TestSSHTool_Execute_RequestsWriteLevel(t *testing.T) {
	var gotLevel string
	spy := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		gotLevel = level
		return false, nil // deny is fine — this test only cares which level was requested
	}

	tool := NewSSHTool(1, &testsupport.StubSSHConnService{}, spy, testsupport.NoopCommandRules{})
	_, _ = tool.Execute(context.Background(), map[string]any{"connectionId": float64(5), "program": "ls"})

	if gotLevel != "write" {
		t.Fatalf("expected SSHTool to request level %q, got %q", "write", gotLevel)
	}
}

func TestSSHTool_Execute_DeniedAccessNeverReachesConnectionLookup(t *testing.T) {
	svc := &testsupport.StubSSHConnService{GetErr: errors.New("GetByID should never be called")}
	tool := NewSSHTool(1, svc, testsupport.DenyAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when CanAccessResource denies")
	}
	if data, ok := result.Data.(string); !ok || data == "" {
		t.Fatalf("expected a non-empty denial reason, got %+v", result.Data)
	}
}

func TestSSHTool_Execute_ConnectionNotFound(t *testing.T) {
	svc := &testsupport.StubSSHConnService{GetErr: shared.ErrNotFound}
	tool := NewSSHTool(1, svc, testsupport.AllowAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when the connection doesn't exist")
	}
}

func TestSSHTool_Execute_UnparseablePrivateKey(t *testing.T) {
	svc := &testsupport.StubSSHConnService{
		Conn:   &sshconn.SSHConnection{Host: "example.com", Port: 22, Username: "u"},
		Secret: "not a real private key",
	}
	tool := NewSSHTool(1, svc, testsupport.AllowAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for an unparseable private key")
	}
}

func TestSSHTool_Execute_ArgsMustBeStringArray(t *testing.T) {
	tool := NewSSHTool(1, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
		"args":         "not-an-array",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when args isn't an array")
	}
}
