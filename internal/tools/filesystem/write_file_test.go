package filesystem_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func TestWriteFileTool_CreatesNewFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	tool := filesystem.NewWriteFileTool(ws, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "new.txt", "content": "hello",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}

	got, err := os.ReadFile(filepath.Join(root, "new.txt"))
	if err != nil {
		t.Fatalf("expected the file to exist: %v", err)
	}
	if string(got) != "hello" {
		t.Fatalf("expected content %q, got %q", "hello", got)
	}
}

func TestWriteFileTool_CreatesParentDirectories(t *testing.T) {
	ws, root := newTestWorkspace(t)
	tool := filesystem.NewWriteFileTool(ws, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a/b/c/new.txt", "content": "nested",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got: %+v", result.Data)
	}
	if _, err := os.Stat(filepath.Join(root, "a", "b", "c", "new.txt")); err != nil {
		t.Fatalf("expected nested parent directories to be created: %v", err)
	}
}

func TestWriteFileTool_WithoutTrackerAllowsOverwritingWithoutReadingFirst(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewWriteFileTool(ws, nil)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "existing.txt", "content": "new",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success with no tracker wired up, got: %+v", result.Data)
	}
}

func TestWriteFileTool_WithTrackerRefusesToOverwriteAnUnreadFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tracker := filesystem.NewReadTracker()
	tool := filesystem.NewWriteFileTool(ws, tracker)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "existing.txt", "content": "new",
	})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure overwriting a file that was never read")
	}

	got, err := os.ReadFile(filepath.Join(root, "existing.txt"))
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if string(got) != "old" {
		t.Fatal("expected the file to be left untouched after the refused overwrite")
	}
}

func TestWriteFileTool_WithTrackerAllowsOverwriteAfterRead(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("old"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tracker := filesystem.NewReadTracker()
	readTool := filesystem.NewReadFileTool(ws, tracker)
	writeTool := filesystem.NewWriteFileTool(ws, tracker)

	if _, err := readTool.Execute(context.Background(), map[string]any{"path": "existing.txt"}); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	result, err := writeTool.Execute(context.Background(), map[string]any{
		"path": "existing.txt", "content": "new",
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success after reading the file first, got: %+v", result.Data)
	}
}

func TestWriteFileTool_RejectsPathEscape(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewWriteFileTool(ws, nil)

	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "../escape.txt", "content": "x",
	})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a path escaping the workspace")
	}
}
