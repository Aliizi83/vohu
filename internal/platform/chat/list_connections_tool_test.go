package chat

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
)

// listStubSSHConnService only implements List — the one method this
// tool calls.
type listStubSSHConnService struct {
	stubSSHConnService
	items []sshconn.SSHConnection
}

func (s *listStubSSHConnService) List(context.Context, shared.DynamicFilter, shared.Pagination) ([]sshconn.SSHConnection, int64, error) {
	return s.items, int64(len(s.items)), nil
}

func TestListSSHConnectionsTool_Execute_OnlyReturnsAccessibleConnections(t *testing.T) {
	svc := &listStubSSHConnService{
		items: []sshconn.SSHConnection{
			{Name: "allowed-box", Host: "1.1.1.1", Username: "u1"},
			{Name: "denied-box", Host: "2.2.2.2", Username: "u2"},
		},
	}
	svc.items[0].ID = 1
	svc.items[1].ID = 2

	canAccess := func(ctx context.Context, userID uint, resourceType string, resourceID uint) (bool, error) {
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
	svc := &listStubSSHConnService{
		items: []sshconn.SSHConnection{{Name: "denied-box", Host: "2.2.2.2"}},
	}
	svc.items[0].ID = 1

	tool := NewListSSHConnectionsTool(7, svc, denyAccess)

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
