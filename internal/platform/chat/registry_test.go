package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var errTestListFailed = errors.New("list failed")

// stubAgentToolService is a minimal agenttool.Service double — only
// ListForCaller is ever reached by buildRegistry.
type stubAgentToolService struct {
	rows []agenttool.Tool
	err  error
}

func (s *stubAgentToolService) GetByID(context.Context, uint) (*agenttool.Tool, error) {
	panic("not used by buildRegistry")
}
func (s *stubAgentToolService) Update(context.Context, uint, agenttool.UpdateToolRequest) (*agenttool.Tool, error) {
	panic("not used by buildRegistry")
}
func (s *stubAgentToolService) List(context.Context, shared.DynamicFilter, shared.Pagination) ([]agenttool.Tool, int64, error) {
	panic("not used by buildRegistry")
}
func (s *stubAgentToolService) ListForCaller(context.Context, uint, shared.DynamicFilter, shared.Pagination) ([]agenttool.Tool, int64, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.rows, int64(len(s.rows)), nil
}

func TestBuildRegistry_RegistersSSHExecute(t *testing.T) {
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{{Name: "ssh_execute"}}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubSSHConnService{}, allowAccess, noopPolicy{})
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if _, ok := registry.Get("ssh_execute"); !ok {
		t.Fatal("expected ssh_execute to be registered")
	}
}

func TestBuildRegistry_RegistersStubToolsForUnimplementedNames(t *testing.T) {
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{
		{Name: "read_file"}, {Name: "write_file"}, {Name: "edit_file"},
		{Name: "list_directory"}, {Name: "search_files"}, {Name: "find_files"},
	}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubSSHConnService{}, allowAccess, noopPolicy{})
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if len(registry.All()) != 6 {
		t.Fatalf("expected all 6 stub tools registered, got %d", len(registry.All()))
	}
	for _, name := range []string{"read_file", "write_file", "edit_file", "list_directory", "search_files", "find_files"} {
		tool, ok := registry.Get(name)
		if !ok {
			t.Fatalf("expected %q to be registered", name)
		}
		if _, isStub := tool.(*StubRemoteTool); !isStub {
			t.Fatalf("expected %q to be a *StubRemoteTool, got %T", name, tool)
		}
	}
}

func TestBuildRegistry_UnknownNameIsSkippedNotFatal(t *testing.T) {
	// A DB row whose name matches no Go implementation (stale after a
	// rename, e.g.) shouldn't take down the whole turn — it's just
	// silently absent from the registry.
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{
		{Name: "does_not_exist_anymore"},
		{Name: "ssh_execute"},
	}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubSSHConnService{}, allowAccess, noopPolicy{})
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if len(registry.All()) != 1 {
		t.Fatalf("expected exactly 1 registered tool (the unknown name skipped), got %d", len(registry.All()))
	}
}

func TestBuildRegistry_NoAccessibleToolsMeansEmptyRegistry(t *testing.T) {
	// This is the access-control guarantee the whole feature is for: if
	// ListForCaller (Visibility + rbac.ResourceAccess) says the caller
	// can't see a tool, it never even reaches the registry, regardless of
	// what a per-connection check inside the tool itself might separately
	// allow.
	agentTools := &stubAgentToolService{rows: nil}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubSSHConnService{}, allowAccess, noopPolicy{})
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if len(registry.All()) != 0 {
		t.Fatalf("expected an empty registry when ListForCaller returns nothing, got %d tools", len(registry.All()))
	}
}

func TestBuildRegistry_PropagatesListForCallerError(t *testing.T) {
	agentTools := &stubAgentToolService{err: errTestListFailed}

	_, err := buildRegistry(context.Background(), 1, agentTools, &stubSSHConnService{}, allowAccess, noopPolicy{})
	if err == nil {
		t.Fatal("expected buildRegistry to propagate a ListForCaller error")
	}
}
