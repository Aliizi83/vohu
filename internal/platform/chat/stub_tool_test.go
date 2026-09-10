package chat

import (
	"context"
	"strings"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
)

func TestStubRemoteTool_Execute_MissingConnectionID(t *testing.T) {
	tool := NewStubRemoteTool("read_file", "d", nil, nil, accessLevelRead, 1, &stubSSHConnService{}, allowAccess)

	result, err := tool.Execute(context.Background(), map[string]any{"path": "/etc/hosts"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when connectionId is missing")
	}
}

func TestStubRemoteTool_Execute_DeniedAccessNeverReportsUnimplemented(t *testing.T) {
	// The security property: access denial has to come back as "access
	// denied," never silently reworded into "not implemented" — an
	// unimplemented tool still must not leak whether a connection the
	// caller can't reach even exists.
	tool := NewStubRemoteTool("read_file", "d", nil, nil, accessLevelRead, 1, &stubSSHConnService{}, denyAccess)

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(5), "path": "/etc/hosts"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when access to the connection is denied")
	}
	msg, _ := result.Data.(string)
	if strings.Contains(msg, "not implemented") {
		t.Fatalf("expected an access-denied message, not the unimplemented stub message, got %q", msg)
	}
}

func TestStubRemoteTool_Execute_AllowedAccessReportsUnimplemented(t *testing.T) {
	tool := NewStubRemoteTool("read_file", "d", nil, nil, accessLevelRead, 1, &stubSSHConnService{}, allowAccess)

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(5), "path": "/etc/hosts"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false — the tool genuinely isn't implemented yet")
	}
	msg, _ := result.Data.(string)
	if !strings.Contains(msg, "not implemented") {
		t.Fatalf("expected the unimplemented stub message once access is confirmed, got %q", msg)
	}
}

func TestStubRemoteTool_Parameters_AlwaysRequiresConnectionID(t *testing.T) {
	tool := NewStubRemoteTool(
		"write_file", "d",
		map[string]ai_model.ToolProperty{"path": {Type: "string"}, "content": {Type: "string"}},
		[]string{"path", "content"}, accessLevelWrite, 1, &stubSSHConnService{}, allowAccess,
	)

	params := tool.Parameters()
	if _, ok := params.Properties["connectionId"]; !ok {
		t.Fatal("expected connectionId to always be a declared parameter")
	}

	required := map[string]bool{}
	for _, r := range params.Required {
		required[r] = true
	}
	for _, want := range []string{"connectionId", "path", "content"} {
		if !required[want] {
			t.Fatalf("expected %q to be required, got %v", want, params.Required)
		}
	}
}
