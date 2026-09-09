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

const maxFindResults = 1000

type FindFilesTool struct {
	ws *Workspace
}

func NewFindFilesTool(ws *Workspace) *FindFilesTool {
	return &FindFilesTool{ws: ws}
}

func (t *FindFilesTool) Name() string { return "find_files" }

func (t *FindFilesTool) Description() string {
	return "Find files by name pattern rather than content — e.g. \"**/*.test.go\" to match at any depth, or \"*.go\" for one directory only. Common heavy directories (node_modules, .git, ...) are skipped."
}

func (t *FindFilesTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"pattern": {
				Type:        "string",
				Description: "Glob pattern to match file paths against, relative to \"path\". \"**\" matches any number of directories, e.g. \"**/*.test.go\".",
			},
			"path": {
				Type:        "string",
				Description: "Directory to search under, relative to the workspace root. Omit or use \".\" for the workspace root.",
			},
		},
		Required: []string{"pattern"},
	}
}

func (t *FindFilesTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	pattern, err := requiredString(args, "pattern")
	if err != nil {
		return failure(err.Error()), nil
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "."
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

	var matches []string
	truncated := false

	err = filepath.Walk(resolved, func(full string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if full == resolved {
			return nil
		}
		if fi.IsDir() {
			if defaultIgnoredDirs[fi.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if len(matches) >= maxFindResults {
			truncated = true
			return filepath.SkipAll
		}

		relToSearchRoot, err := filepath.Rel(resolved, full)
		if err != nil {
			return err
		}

		matched, err := matchGlob(pattern, filepath.ToSlash(relToSearchRoot))
		if err != nil {
			return fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if !matched {
			return nil
		}

		relToWorkspace, err := filepath.Rel(t.ws.Root(), full)
		if err != nil {
			relToWorkspace = full
		}
		matches = append(matches, filepath.ToSlash(relToWorkspace))
		return nil
	})
	if err != nil {
		return failure(fmt.Sprintf("failed to search %s: %v", path, err)), nil
	}

	sort.Strings(matches)

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"pattern":   pattern,
			"matches":   matches,
			"count":     len(matches),
			"truncated": truncated,
		},
	}, nil
}
