package tools

import (
	"context"
	"testing"

	"github.com/Aliizi83/vohu/internal/ai_model"
)

type fakeTool struct {
	name string
}

func (f fakeTool) Name() string        { return f.name }
func (f fakeTool) Description() string { return "a fake tool named " + f.name }
func (f fakeTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{Required: []string{"x"}}
}
func (f fakeTool) Execute(ctx context.Context, args map[string]any) (ToolResult, error) {
	return ToolResult{Success: true, Data: f.name}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeTool{name: "alpha"})

	got, ok := r.Get("alpha")
	if !ok {
		t.Fatal("expected alpha to be registered")
	}
	if got.Name() != "alpha" {
		t.Fatalf("expected name alpha, got %s", got.Name())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("does-not-exist")
	if ok {
		t.Fatal("expected unknown tool lookup to fail")
	}
}

func TestRegistry_All(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeTool{name: "alpha"})
	r.Register(fakeTool{name: "beta"})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}
}

func TestRegistry_Definitions(t *testing.T) {
	r := NewRegistry()
	r.Register(fakeTool{name: "alpha"})

	defs := r.Definitions()
	if len(defs) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(defs))
	}

	def := defs[0]
	if def.Name != "alpha" {
		t.Fatalf("expected name alpha, got %s", def.Name)
	}
	if def.Description == "" {
		t.Fatal("expected a non-empty description")
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "x" {
		t.Fatalf("expected Parameters to be carried through from the tool, got %+v", def.Parameters)
	}
}
