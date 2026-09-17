package list_connections

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/chat/system_tools/testsupport"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
)

func TestListSSHConnectionsTool_Execute_RequestsReadLevel(t *testing.T) {
	svc := &testsupport.StubSSHConnService{Items: []sshconn.SSHConnection{{Name: "box", Host: "1.1.1.1"}}}
	svc.Items[0].ID = 1

	var gotLevel string
	spy := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		gotLevel = level
		return true, nil
	}

	tool := NewListSSHConnectionsTool(7, svc, spy)
	if _, err := tool.Execute(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotLevel != "read" {
		t.Fatalf("expected ListSSHConnectionsTool to request level %q, got %q", "read", gotLevel)
	}
}

func TestListSSHConnectionsTool_Execute_OnlyReturnsAccessibleConnections(t *testing.T) {
	svc := &testsupport.StubSSHConnService{
		Items: []sshconn.SSHConnection{
			{Name: "allowed-box", Host: "1.1.1.1", Username: "u1"},
			{Name: "denied-box", Host: "2.2.2.2", Username: "u2"},
		},
	}
	svc.Items[0].ID = 1
	svc.Items[1].ID = 2

	canAccess := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		return resourceID == 1, nil // only connection 1 is visible to this user
	}

	tool := NewListSSHConnectionsTool(7, svc, canAccess)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}

	visible, ok := result.Data.([]map[string]any)
	if !ok {
		t.Fatalf("expected []map[string]any, got %T", result.Data)
	}
	if len(visible) != 1 {
		t.Fatalf("expected exactly 1 visible connection, got %d: %+v", len(visible), visible)
	}
	if visible[0]["name"] != "allowed-box" {
		t.Fatalf("expected the allowed connection, got %+v", visible[0])
	}
}

func TestListSSHConnectionsTool_Execute_NoAccessibleConnectionsReturnsEmptyNotNil(t *testing.T) {
	svc := &testsupport.StubSSHConnService{
		Items: []sshconn.SSHConnection{{Name: "denied-box", Host: "2.2.2.2"}},
	}
	svc.Items[0].ID = 1

	tool := NewListSSHConnectionsTool(7, svc, testsupport.DenyAccess)

	result, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success even with zero visible connections")
	}
	visible, ok := result.Data.([]map[string]any)
	if !ok || len(visible) != 0 {
		t.Fatalf("expected an empty slice, got %+v", result.Data)
	}
}
