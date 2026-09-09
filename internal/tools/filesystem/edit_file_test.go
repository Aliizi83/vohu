package filesystem_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func TestEditFileTool_ReplacesTheUniqueMatch(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("func Foo() {\n\treturn 1\n}\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewEditFileTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a.go", "old_str": "return 1", "new_str": "return 2",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	got, err := os.ReadFile(filepath.Join(root, "a.go"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	want := "func Foo() {\n\treturn 2\n}\n"
	if string(got) != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestEditFileTool_FailsWhenOldStrNotFound(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewEditFileTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a.txt", "old_str": "goodbye", "new_str": "x",
	})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when old_str isn't present")
	}
}

func TestEditFileTool_FailsWhenOldStrMatchesMultipleTimes(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("foo\nfoo\n"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewEditFileTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a.txt", "old_str": "foo", "new_str": "bar",
	})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when old_str matches more than once")
	}

	got, err := os.ReadFile(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != "foo\nfoo\n" {
		t.Fatal("expected the file to be left untouched when the match is ambiguous")
	}
}

func TestEditFileTool_FailsOnNoOpReplacement(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("same"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewEditFileTool(ws)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a.txt", "old_str": "same", "new_str": "same",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when old_str and new_str are identical")
	}
}
