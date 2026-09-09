package filesystem_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func newTestWorkspace(t *testing.T) (*filesystem.Workspace, string) {
	t.Helper()
	root := t.TempDir()
	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}
	return ws, root
}

func TestReadFileTool_ReturnsNumberedLines(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("one\ntwo\nthree"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewReadFileTool(ws, nil)
	result, err := tool.Execute(context.Background(), map[string]any{"path": "a.txt"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got failure: %+v", result.Data)
	}

	data := result.Data.(map[string]any)
	content := data["content"].(string)
	if !strings.Contains(content, "1\tone") || !strings.Contains(content, "3\tthree") {
		t.Fatalf("expected cat -n style numbering, got:\n%s", content)
	}
	if data["totalLines"].(int) != 3 {
		t.Fatalf("expected totalLines=3, got %v", data["totalLines"])
	}
}

func TestReadFileTool_OffsetAndLimitPageThroughTheFile(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("l1\nl2\nl3\nl4\nl5"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewReadFileTool(ws, nil)
	result, err := tool.Execute(context.Background(), map[string]any{
		"path": "a.txt", "offset": float64(2), "limit": float64(2),
	})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	data := result.Data.(map[string]any)
	content := data["content"].(string)

	if !strings.Contains(content, "2\tl2") || !strings.Contains(content, "3\tl3") {
		t.Fatalf("expected lines 2-3 only, got:\n%s", content)
	}
	if strings.Contains(content, "l1") || strings.Contains(content, "l4") {
		t.Fatalf("expected lines outside the offset/limit window to be excluded, got:\n%s", content)
	}
}

func TestReadFileTool_NotFound(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewReadFileTool(ws, nil)

	result, err := tool.Execute(context.Background(), map[string]any{"path": "missing.txt"})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a missing file")
	}
}

func TestReadFileTool_DetectsBinaryFiles(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 0x01, 0x02, 'h', 'i'}, 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tool := filesystem.NewReadFileTool(ws, nil)
	result, err := tool.Execute(context.Background(), map[string]any{"path": "bin.dat"})
	if err != nil {
		t.Fatalf("Execute returned an error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success (a recognized-as-binary response, not a failure), got: %+v", result.Data)
	}
	data := result.Data.(map[string]any)
	if binary, _ := data["binary"].(bool); !binary {
		t.Fatalf("expected binary=true, got %+v", data)
	}
}

func TestReadFileTool_RejectsPathEscape(t *testing.T) {
	ws, _ := newTestWorkspace(t)
	tool := filesystem.NewReadFileTool(ws, nil)

	result, err := tool.Execute(context.Background(), map[string]any{"path": "../../etc/passwd"})
	if err != nil {
		t.Fatalf("Execute returned a Go error rather than a failed ToolResult: %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for a path escaping the workspace")
	}
}

func TestReadFileTool_MarksPathReadInTracker(t *testing.T) {
	ws, root := newTestWorkspace(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	tracker := filesystem.NewReadTracker()
	tool := filesystem.NewReadFileTool(ws, tracker)

	if _, err := tool.Execute(context.Background(), map[string]any{"path": "a.txt"}); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	resolved := filepath.Join(root, "a.txt")
	if !tracker.WasRead(resolved) {
		t.Fatal("expected the tracker to mark a.txt as read")
	}
}
