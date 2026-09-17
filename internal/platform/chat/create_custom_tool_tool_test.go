package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/toolbuild"
	"github.com/Aliizi83/vohu/internal/tools"
)

// stubBuilder lets a test script whether toolbuild.Builder.Build succeeds
// or fails, without invoking a real Go toolchain.
type stubBuilder struct {
	err error
}

func (b *stubBuilder) Build(context.Context, toolbuild.Request) ([]byte, error) {
	if b.err != nil {
		return nil, b.err
	}
	return []byte("binary"), nil
}

// creatingCustomToolService is a customtool.Service double whose
// CreateTool/CreateVersion actually record what they were called with —
// stubCustomToolService (registry_test.go) panics on both, since
// buildRegistry itself never reaches them.
type creatingCustomToolService struct {
	createErr  error
	versionErr error

	createdTool    *customtool.CreateToolRequest
	createdVersion *customtool.CreateVersionRequest
}

func (s *creatingCustomToolService) CreateTool(_ context.Context, _ uint, req customtool.CreateToolRequest) (*customtool.Tool, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.createdTool = &req
	return &customtool.Tool{Name: req.Name, Description: req.Description, ParamsSchema: req.ParamsSchema, Visibility: req.Visibility}, nil
}
func (s *creatingCustomToolService) GetToolByID(context.Context, uint) (*customtool.Tool, error) {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) UpdateTool(context.Context, uint, customtool.UpdateToolRequest) (*customtool.Tool, error) {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) DeleteTool(context.Context, uint) error {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) ListTools(context.Context, shared.DynamicFilter, shared.Pagination) ([]customtool.Tool, int64, error) {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) ListToolsForCaller(context.Context, uint, shared.DynamicFilter, shared.Pagination) ([]customtool.Tool, int64, error) {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) CreateVersion(_ context.Context, _ uint, _ uint, req customtool.CreateVersionRequest) (*customtool.ToolVersion, error) {
	if s.versionErr != nil {
		return nil, s.versionErr
	}
	s.createdVersion = &req
	return &customtool.ToolVersion{Version: req.Version, SourceCode: req.SourceCode}, nil
}
func (s *creatingCustomToolService) ListVersionsForTool(context.Context, uint, shared.Pagination) ([]customtool.ToolVersion, int64, error) {
	panic("not used by CreateCustomToolTool")
}
func (s *creatingCustomToolService) LatestVersionForTool(context.Context, uint) (*customtool.ToolVersion, error) {
	panic("not used by CreateCustomToolTool")
}

const validParamsSchema = `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`

func newTestCreateCustomToolTool(canAccess func(context.Context, uint, string, uint, string) (bool, error), builderErr error, customTools *creatingCustomToolService, registry *tools.Registry) *CreateCustomToolTool {
	return NewCreateCustomToolTool(1, canAccess, customTools, &stubBuilder{err: builderErr}, &stubSSHConnService{}, nil, 0, registry)
}

func TestCreateCustomToolTool_Execute_MissingRequiredFields(t *testing.T) {
	tool := newTestCreateCustomToolTool(allowAccess, nil, &creatingCustomToolService{}, tools.NewRegistry())

	result, err := tool.Execute(context.Background(), map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when required fields are missing")
	}
}

func TestCreateCustomToolTool_Execute_InvalidParamsSchema(t *testing.T) {
	tool := newTestCreateCustomToolTool(allowAccess, nil, &creatingCustomToolService{}, tools.NewRegistry())

	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "my_tool", "description": "does a thing", "paramsSchema": "not json", "sourceCode": "package main",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false for a malformed paramsSchema")
	}
}

func TestCreateCustomToolTool_Execute_AccessDeniedNeverReachesBuilder(t *testing.T) {
	tool := newTestCreateCustomToolTool(denyAccess, errors.New("Build should never be called"), &creatingCustomToolService{}, tools.NewRegistry())

	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "my_tool", "description": "does a thing", "paramsSchema": validParamsSchema, "sourceCode": "package main",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when access is denied")
	}
}

func TestCreateCustomToolTool_Execute_CompileFailureReturnsDiagnosticsNotCreated(t *testing.T) {
	customTools := &creatingCustomToolService{}
	tool := newTestCreateCustomToolTool(allowAccess, errors.New("building: exit status 1: ./main.go:3:2: undefined: fmt"), customTools, tools.NewRegistry())

	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "my_tool", "description": "does a thing", "paramsSchema": validParamsSchema, "sourceCode": "package main",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected Success=false when the source doesn't compile")
	}
	if customTools.createdTool != nil {
		t.Fatal("expected CreateTool to never be called for source that fails to compile")
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["compileError"] != true {
		t.Fatalf("expected compileError=true in result data, got %+v", result.Data)
	}
}

func TestCreateCustomToolTool_Execute_SuccessCreatesAndRegistersLive(t *testing.T) {
	customTools := &creatingCustomToolService{}
	registry := tools.NewRegistry()
	tool := newTestCreateCustomToolTool(allowAccess, nil, customTools, registry)

	result, err := tool.Execute(context.Background(), map[string]any{
		"name": "my_new_tool", "description": "does a thing", "paramsSchema": validParamsSchema, "sourceCode": "package main\nfunc main() {}",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.Success {
		t.Fatalf("expected Success=true, got Data=%+v", result.Data)
	}
	if customTools.createdTool == nil || customTools.createdTool.Name != "my_new_tool" {
		t.Fatalf("expected CreateTool to be called with the tool's name, got %+v", customTools.createdTool)
	}
	if customTools.createdVersion == nil || customTools.createdVersion.SourceCode == "" {
		t.Fatal("expected CreateVersion to be called with the source code")
	}

	registered, ok := registry.Get("my_new_tool")
	if !ok {
		t.Fatal("expected the new tool to be registered immediately")
	}
	if _, isCustom := registered.(*CustomTool); !isCustom {
		t.Fatalf("expected the registered tool to be a *CustomTool, got %T", registered)
	}
}
