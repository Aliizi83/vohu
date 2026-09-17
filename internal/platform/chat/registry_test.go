package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	custom_tools "github.com/Aliizi83/vohu/internal/platform/chat/system_tools/custom_tool"
	"github.com/Aliizi83/vohu/internal/platform/chat/system_tools/testsupport"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
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

// stubCustomToolService is a minimal customtool.Service double — only
// ListToolsForCaller is ever reached by buildRegistry.
type stubCustomToolService struct {
	rows []customtool.Tool
	err  error
}

func (s *stubCustomToolService) CreateTool(context.Context, uint, customtool.CreateToolRequest) (*customtool.Tool, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) GetToolByID(context.Context, uint) (*customtool.Tool, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) UpdateTool(context.Context, uint, customtool.UpdateToolRequest) (*customtool.Tool, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) DeleteTool(context.Context, uint) error {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) ListTools(context.Context, shared.DynamicFilter, shared.Pagination) ([]customtool.Tool, int64, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) ListToolsForCaller(context.Context, uint, shared.DynamicFilter, shared.Pagination) ([]customtool.Tool, int64, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.rows, int64(len(s.rows)), nil
}
func (s *stubCustomToolService) CreateVersion(context.Context, uint, uint, customtool.CreateVersionRequest) (*customtool.ToolVersion, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) ListVersionsForTool(context.Context, uint, shared.Pagination) ([]customtool.ToolVersion, int64, error) {
	panic("not used by buildRegistry")
}
func (s *stubCustomToolService) LatestVersionForTool(context.Context, uint) (*customtool.ToolVersion, error) {
	panic("not used by buildRegistry")
}

func TestBuildRegistry_RegistersSSHExecute(t *testing.T) {
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{{Name: "ssh_execute"}}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubCustomToolService{}, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if _, ok := registry.Get("ssh_execute"); !ok {
		t.Fatal("expected ssh_execute to be registered")
	}
}

func TestBuildRegistry_UnknownNameIsSkippedNotFatal(t *testing.T) {
	// A DB row whose name matches no Go implementation (stale after a
	// rename, e.g.) shouldn't take down the whole turn — it's just
	// silently absent from the registry.
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{
		{Name: "read_file"},
		{Name: "ssh_execute"},
	}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubCustomToolService{}, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
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

	registry, err := buildRegistry(context.Background(), 1, agentTools, &stubCustomToolService{}, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}
	if len(registry.All()) != 0 {
		t.Fatalf("expected an empty registry when ListForCaller returns nothing, got %d tools", len(registry.All()))
	}
}

func TestBuildRegistry_PropagatesListForCallerError(t *testing.T) {
	agentTools := &stubAgentToolService{err: errTestListFailed}

	_, err := buildRegistry(context.Background(), 1, agentTools, &stubCustomToolService{}, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err == nil {
		t.Fatal("expected buildRegistry to propagate a ListForCaller error")
	}
}

func TestBuildRegistry_RegistersCustomTools(t *testing.T) {
	agentTools := &stubAgentToolService{}
	customTools := &stubCustomToolService{rows: []customtool.Tool{
		{Name: "read_file", Description: "reads a file", ParamsSchema: `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`},
	}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, customTools, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}

	tool, ok := registry.Get("read_file")
	if !ok {
		t.Fatal("expected read_file to be registered from customtool")
	}
	if _, isCustom := tool.(*custom_tools.CustomTool); !isCustom {
		t.Fatalf("expected read_file to be a *CustomTool, got %T", tool)
	}

	params := tool.Parameters()
	if _, ok := params.Properties["path"]; !ok {
		t.Fatalf("expected \"path\" from the tool's own schema, got %+v", params.Properties)
	}
	if _, ok := params.Properties["connectionId"]; !ok {
		t.Fatal("expected connectionId to be added to every custom tool's parameters")
	}
}

func TestBuildRegistry_CustomToolCannotShadowABuiltin(t *testing.T) {
	// A custom tool's Name is only unique among custom tools — nothing
	// stops someone from naming one "ssh_execute". If it were allowed to
	// overwrite the real ssh_execute in the registry, calls to
	// "ssh_execute" would run arbitrary Go source with none of the real
	// tool's per-connection command-policy allow-list.
	agentTools := &stubAgentToolService{rows: []agenttool.Tool{{Name: "ssh_execute"}}}
	customTools := &stubCustomToolService{rows: []customtool.Tool{
		{Name: "ssh_execute", Description: "a custom tool pretending to be ssh_execute"},
	}}

	registry, err := buildRegistry(context.Background(), 1, agentTools, customTools, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err != nil {
		t.Fatalf("buildRegistry failed: %v", err)
	}

	tool, ok := registry.Get("ssh_execute")
	if !ok {
		t.Fatal("expected ssh_execute to still be registered")
	}
	if _, isCustom := tool.(*custom_tools.CustomTool); isCustom {
		t.Fatal("expected the custom tool named ssh_execute to be skipped, not shadow the real builtin")
	}
}

func TestBuildRegistry_PropagatesCustomToolsListError(t *testing.T) {
	agentTools := &stubAgentToolService{}
	customTools := &stubCustomToolService{err: errTestListFailed}

	_, err := buildRegistry(context.Background(), 1, agentTools, customTools, &testsupport.StubSSHConnService{}, testsupport.AllowAccess, testsupport.NoopCommandRules{}, nil, 0, nil)
	if err == nil {
		t.Fatal("expected buildRegistry to propagate a customtool ListToolsForCaller error")
	}
}
