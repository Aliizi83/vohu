package filesystem

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

// defaultIgnoredDirs are skipped during a recursive listing unless the
// caller asks for something inside them by an exact path — they're near
// always noise (dependency trees, VCS metadata) and can be enormous.
var defaultIgnoredDirs = map[string]bool{
	"node_modules": true,
	".git":         true,
	"venv":         true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	"build":        true,
}

const maxListEntries = 2000

type ListDirectoryTool struct {
	ws *Workspace
}

func NewListDirectoryTool(ws *Workspace) *ListDirectoryTool {
	return &ListDirectoryTool{ws: ws}
}

func (t *ListDirectoryTool) Name() string { return "list_directory" }

func (t *ListDirectoryTool) Description() string {
	return "List files and directories at a path. Common heavy directories (node_modules, .git, venv, ...) are skipped by default when listing recursively."
}

func (t *ListDirectoryTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"path": {
				Type:        "string",
				Description: "Directory to list, relative to the workspace root. Omit or use \".\" for the workspace root.",
			},
			"recursive": {
				Type:        "boolean",
				Description: "List subdirectories' contents too. Defaults to false (one level only).",
			},
			"max_depth": {
				Type:        "integer",
				Description: "Maximum recursion depth when recursive is true. Omit for no limit.",
			},
		},
	}
}

type entry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

func (t *ListDirectoryTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	recursive := optionalBool(args, "recursive")
	maxDepth, hasMaxDepth, err := optionalInt(args, "max_depth")
	if err != nil {
		return failure(err.Error()), nil
	}

	resolved, err := t.ws.Resolve(path)
	if err != nil {
		return failure(err.Error()), nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return failure(fmt.Sprintf("directory not found: %s", path)), nil
		}
		return failure(fmt.Sprintf("failed to stat %s: %v", path, err)), nil
	}
	if !info.IsDir() {
		return failure(fmt.Sprintf("%s is not a directory", path)), nil
	}

	var entries []entry
	truncated := false

	var walk func(dir string, depth int) error
	walk = func(dir string, depth int) error {
		items, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })

		for _, item := range items {
			if len(entries) >= maxListEntries {
				truncated = true
				return nil
			}

			full := filepath.Join(dir, item.Name())
			rel, err := filepath.Rel(t.ws.Root(), full)
			if err != nil {
				rel = full
			}

			entries = append(entries, entry{Path: rel, IsDir: item.IsDir()})

			if item.IsDir() && recursive && !defaultIgnoredDirs[item.Name()] {
				if !hasMaxDepth || depth < maxDepth {
					if err := walk(full, depth+1); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}

	if err := walk(resolved, 1); err != nil {
		return failure(fmt.Sprintf("failed to list %s: %v", path, err)), nil
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"path":      path,
			"entries":   entries,
			"count":     len(entries),
			"truncated": truncated,
		},
	}, nil
}
