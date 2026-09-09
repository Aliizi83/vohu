package filesystem_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func decodeMatches(t *testing.T, data map[string]any) []map[string]any {
	t.Helper()
	encoded, err := json.Marshal(data["matches"])
	if err != nil {
		t.Fatalf("failed to marshal matches: %v", err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal matches: %v", err)
	}
	return decoded
}

func TestSearchFilesTool_FindsMatchesAcrossFiles(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("func Foo() {}\nfunc Bar() {}\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.go"), []byte("func Baz() {}\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewSearchFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "func (Foo|Baz)"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	matches := decodeMatches(t, data)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d: %+v", len(matches), matches)
	}

	found := map[string]bool{}
	for _, m := range matches {
		found[m["path"].(string)] = true
		if int(m["line"].(float64)) != 1 {
			t.Errorf("expected line 1, got %v", m["line"])
		}
	}
	if !found["a.go"] || !found["b.go"] {
		t.Fatalf("expected matches in both a.go and b.go, got %+v", matches)
	}
}

func TestSearchFilesTool_RespectsFileGlob(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("needle"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewSearchFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"pattern": "needle", "file_glob": "*.go",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	matches := decodeMatches(t, data)

	if len(matches) != 1 || matches[0]["path"].(string) != "a.go" {
		t.Fatalf("expected only a.go to match with file_glob=*.go, got %+v", matches)
	}
}

func TestSearchFilesTool_CaseSensitivityDefaultsToTrue(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("Needle"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewSearchFilesTool(ws)

	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["count"].(int) != 0 {
		t.Fatalf("expected no case-sensitive match for lowercase 'needle' against 'Needle', got %+v", data)
	}

	result, err = tool.Execute(context.Background(), map[string]any{"pattern": "needle", "case_sensitive": false})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data = result.Data.(map[string]any)
	if data["count"].(int) != 1 {
		t.Fatalf("expected a case-insensitive match, got %+v", data)
	}
}

func TestSearchFilesTool_SkipsBinaryFiles(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 'n', 'e', 'e', 'd', 'l', 'e'}, 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewSearchFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "needle"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["count"].(int) != 0 {
		t.Fatalf("expected binary files to be skipped, got %+v", data)
	}
}

func TestSearchFilesTool_InvalidPatternFails(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewSearchFilesTool(ws)

	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "("})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for an invalid regex pattern")
	}
}

func TestSearchFilesTool_SearchesASingleFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("needle here"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("needle here too"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewSearchFilesTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{"pattern": "needle", "path": "a.txt"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["count"].(int) != 1 {
		t.Fatalf("expected exactly 1 match scoped to a.txt, got %+v", data)
	}
}
