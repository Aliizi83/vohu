package filesystem

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

type EditFileTool struct {
	ws *Workspace
}

func NewEditFileTool(ws *Workspace) *EditFileTool {
	return &EditFileTool{ws: ws}
}

func (t *EditFileTool) Name() string { return "edit_file" }

func (t *EditFileTool) Description() string {
	return "Replace one exact block of text in an existing file. old_str must match exactly once in the file — include enough surrounding context to make it unique. Safer than write_file for a small change since it doesn't rewrite the whole file."
}

func (t *EditFileTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"path": {
				Type:        "string",
				Description: "Path to the file, relative to the workspace root.",
			},
			"old_str": {
				Type:        "string",
				Description: "The exact text to replace. Must match exactly once in the file, whitespace included.",
			},
			"new_str": {
				Type:        "string",
				Description: "The text to replace it with.",
			},
		},
		Required: []string{"path", "old_str", "new_str"},
	}
}

func (t *EditFileTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	path, err := requiredString(args, "path")
	if err != nil {
		return failure(err.Error()), nil
	}

	oldStr, ok := args["old_str"].(string)
	if !ok || oldStr == "" {
		return failure(`"old_str" is required`), nil
	}

	newStr, ok := args["new_str"].(string)
	if !ok {
		return failure(`"new_str" is required`), nil
	}

	if oldStr == newStr {
		return failure(`"old_str" and "new_str" are identical`), nil
	}

	resolved, err := t.ws.Resolve(path)
	if err != nil {
		return failure(err.Error()), nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return failure(fmt.Sprintf("file not found: %s", path)), nil
		}
		return failure(fmt.Sprintf("failed to read %s: %v", path, err)), nil
	}
	content := string(data)

	count := strings.Count(content, oldStr)
	switch count {
	case 0:
		return failure(fmt.Sprintf("old_str not found in %s — it must match the file's content exactly, including whitespace", path)), nil
	case 1:
		// exactly one match, proceed
	default:
		return failure(fmt.Sprintf("old_str matches %d times in %s — add more surrounding context so it matches exactly once", count, path)), nil
	}

	updated := strings.Replace(content, oldStr, newStr, 1)

	if err := os.WriteFile(resolved, []byte(updated), 0o644); err != nil {
		return failure(fmt.Sprintf("failed to write %s: %v", path, err)), nil
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"path":    path,
			"message": fmt.Sprintf("edited %s", path),
		},
	}, nil
}
