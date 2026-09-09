package filesystem_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func setupListingFixture(t *testing.T, root string) {
	t.Helper()
	must := func(err error) {
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
	}
	must(os.WriteFile(filepath.Join(root, "a.txt"), []byte("a"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "src"), 0o755))
	must(os.WriteFile(filepath.Join(root, "src", "main.go"), []byte("x"), 0o644))
	must(os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755))
	must(os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("x"), 0o644))
}

// entryPaths marshals result.Data's entries field to JSON and back into
// a plain []map[string]any, so the test can inspect them without needing
// access to the package's unexported entry type.
func entryPaths(t *testing.T, data map[string]any) []string {
	t.Helper()

	encoded, err := json.Marshal(data["entries"])
	if err != nil {
		t.Fatalf("failed to marshal entries: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal entries: %v", err)
	}

	paths := make([]string, 0, len(decoded))
	for _, e := range decoded {
		paths = append(paths, e["path"].(string))
	}
	return paths
}

func TestListDirectoryTool_NonRecursiveListsOneLevel(t *testing.T) {
	ws, root := newTestWorkspace(t)
	setupListingFixture(t, root)

	tool := filesystem.NewListDirectoryTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	if data["count"].(int) != 3 { // a.txt, src, node_modules — one level only
		t.Fatalf("expected 3 top-level entries, got %v (%+v)", data["count"], data["entries"])
	}

	paths := entryPaths(t, data)
	for _, want := range []string{"a.txt", "src", "node_modules"} {
		found := false
		for _, p := range paths {
			if p == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected %q among top-level entries, got %v", want, paths)
		}
	}
}

func TestListDirectoryTool_RecursiveSkipsNodeModulesByDefault(t *testing.T) {
	ws, root := newTestWorkspace(t)
	setupListingFixture(t, root)

	tool := filesystem.NewListDirectoryTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"recursive": true})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}

	data := result.Data.(map[string]any)
	paths := entryPaths(t, data)

	for _, p := range paths {
		if strings.Contains(p, "node_modules") && p != "node_modules" {
			t.Fatalf("expected node_modules' contents to be skipped, found %q", p)
		}
	}

	sawSrcFile := false
	for _, p := range paths {
		if p == filepath.Join("src", "main.go") {
			sawSrcFile = true
		}
	}
	if !sawSrcFile {
		t.Fatalf("expected recursive listing to reach src/main.go, got %v", paths)
	}
}

func TestListDirectoryTool_MaxDepthLimitsRecursion(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.MkdirAll(filepath.Join(root, "a", "b", "c"), 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "b", "c", "deep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewListDirectoryTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"recursive": true, "max_depth": float64(1),
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}

	data := result.Data.(map[string]any)
	paths := entryPaths(t, data)
	for _, p := range paths {
		if strings.Contains(p, "deep.txt") {
			t.Fatalf("expected max_depth=1 to stop before reaching deep.txt, got %v", paths)
		}
	}
}

func TestListDirectoryTool_NotFound(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewListDirectoryTool(ws)

	result, err := tool.Execute(context.Background(), map[string]any{"path": "missing"})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a missing directory")
	}
}
