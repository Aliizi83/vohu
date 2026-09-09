package filesystem_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func setupFindFixture(t *testing.T, root string) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	must(os.MkdirAll(filepath.Join(root, "internal", "pkg"), 0o755))
	must(os.WriteFile(filepath.Join(root, "main.go"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "internal", "a.go"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "internal", "a_test.go"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "internal", "pkg", "b_test.go"), []byte("x"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "node_modules", "dep"), 0o755))
	must(os.WriteFile(filepath.Join(root, "node_modules", "dep", "index_test.go"), []byte("x"), 0o644))
}

func TestFindFilesTool_MatchesRecursivelyWithDoubleStar(t *testing.T) {
	ws, root := newTestWorkspace(t)
	setupFindFixture(t, root)

	tool := filesystem.NewFindFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "**/*_test.go"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	matches := data["matches"].([]string)

	want := map[string]bool{
		"internal/a_test.go":     true,
		"internal/pkg/b_test.go": true,
	}
	got := map[string]bool{}
	for _, m := range matches {
		got[m] = true
	}
	for path := range want {
		if !got[path] {
			t.Errorf("expected to find %q, got %v", path, matches)
		}
	}
	if got["node_modules/dep/index_test.go"] {
		t.Error("expected node_modules to be skipped by default")
	}
	if got["main.go"] || got["internal/a.go"] {
		t.Errorf("expected only *_test.go files to match, got %v", matches)
	}
}

func TestFindFilesTool_NonDoubleStarPatternIsOneLevelOnly(t *testing.T) {
	ws, root := newTestWorkspace(t)
	setupFindFixture(t, root)

	tool := filesystem.NewFindFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "*.go"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	matches := data["matches"].([]string)

	if len(matches) != 1 || matches[0] != "main.go" {
		t.Fatalf("expected only main.go (top-level, no **), got %v", matches)
	}
}

func TestFindFilesTool_ScopedToGivenPath(t *testing.T) {
	ws, root := newTestWorkspace(t)
	setupFindFixture(t, root)

	tool := filesystem.NewFindFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "**/*_test.go", "path": "internal",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	matches := data["matches"].([]string)

	for _, m := range matches {
		if m == "node_modules/dep/index_test.go" {
			t.Fatal("expected the search to stay scoped to the given path")
		}
	}
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches under internal/, got %v", matches)
	}
}

func TestFindFilesTool_NotFound(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewFindFilesTool(ws)

	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "*.go", "path": "missing",
	})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a missing directory")
	}
}
