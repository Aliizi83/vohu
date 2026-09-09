package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

type WriteFileTool struct {
	ws      *Workspace
	tracker *ReadTracker
}

// NewWriteFileTool: see NewReadFileTool's doc comment for how tracker
// wires the two tools together. With tracker nil, overwriting an
// existing file is always allowed — the "must read first" guard only
// exists when a tracker is actually shared with a ReadFileTool.
func NewWriteFileTool(ws *Workspace, tracker *ReadTracker) *WriteFileTool {
	return &WriteFileTool{ws: ws, tracker: tracker}
}

func (t *WriteFileTool) Name() string { return "write_file" }

func (t *WriteFileTool) Description() string {
	return "Create a new file or fully overwrite an existing one. An existing file must be read with read_file first in this conversation, to avoid accidental overwrites — use edit_file for a small change instead."
}

func (t *WriteFileTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"path": {
				Type:        "string",
				Description: "Path to the file, relative to the workspace root. Parent directories are created if needed.",
			},
			"content": {
				Type:        "string",
				Description: "The full content to write.",
			},
		},
		Required: []string{"path", "content"},
	}
}

func (t *WriteFileTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	path, err := requiredString(args, "path")
	if err != nil {
		return failure(err.Error()), nil
	}

	content, ok := args["content"].(string)
	if !ok {
		return failure(`"content" is required`), nil
	}

	resolved, err := t.ws.Resolve(path)
	if err != nil {
		return failure(err.Error()), nil
	}

	existed := false
	if _, statErr := os.Stat(resolved); statErr == nil {
		existed = true
		if t.tracker != nil && !t.tracker.WasRead(resolved) {
			return failure(fmt.Sprintf(
				"%s already exists and hasn't been read in this conversation yet — read it first with read_file so you know what you're overwriting, or use edit_file for a small change",
				path,
			)), nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return failure(fmt.Sprintf("failed to create parent directories for %s: %v", path, err)), nil
	}

	if err := os.WriteFile(resolved, []byte(content), 0o644); err != nil {
		return failure(fmt.Sprintf("failed to write %s: %v", path, err)), nil
	}

	action := "created"
	if existed {
		action = "overwritten"
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"path":    path,
			"action":  action,
			"bytes":   len(content),
			"message": fmt.Sprintf("%s %s", path, action),
		},
	}, nil
}
