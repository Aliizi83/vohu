package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

const (
	maxSearchMatches  = 500
	maxSearchLineChar = 500
)

type SearchFilesTool struct {
	ws *Workspace
}

func NewSearchFilesTool(ws *Workspace) *SearchFilesTool {
	return &SearchFilesTool{ws: ws}
}

func (t *SearchFilesTool) Name() string { return "search_files" }

func (t *SearchFilesTool) Description() string {
	return "Search for a regex pattern across files' content — faster than reading files one by one to grep through them yourself. Binary files and common heavy directories (node_modules, .git, ...) are skipped."
}

func (t *SearchFilesTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"pattern": {
				Type:        "string",
				Description: "Regular expression to search for (RE2 syntax, Go's regexp package).",
			},
			"path": {
				Type:        "string",
				Description: "File or directory to search, relative to the workspace root. Omit or use \".\" for the whole workspace.",
			},
			"file_glob": {
				Type:        "string",
				Description: "Only search files whose name matches this glob, e.g. \"*.go\". Omit to search every text file.",
			},
			"case_sensitive": {
				Type:        "boolean",
				Description: "Defaults to true.",
			},
		},
		Required: []string{"pattern"},
	}
}

type searchMatch struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func (t *SearchFilesTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	pattern, err := requiredString(args, "pattern")
	if err != nil {
		return failure(err.Error()), nil
	}

	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}

	fileGlob, _ := args["file_glob"].(string)

	caseSensitive := true
	if raw, exists := args["case_sensitive"]; exists && raw != nil {
		b, ok := raw.(bool)
		if !ok {
			return failure(`"case_sensitive" must be a boolean`), nil
		}
		caseSensitive = b
	}

	finalPattern := pattern
	if !caseSensitive {
		finalPattern = "(?i)" + pattern
	}
	re, err := regexp.Compile(finalPattern)
	if err != nil {
		return failure(fmt.Sprintf("invalid pattern %q: %v", pattern, err)), nil
	}

	resolved, err := t.ws.Resolve(path)
	if err != nil {
		return failure(err.Error()), nil
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return failure(fmt.Sprintf("path not found: %s", path)), nil
		}
		return failure(fmt.Sprintf("failed to stat %s: %v", path, err)), nil
	}

	var matches []searchMatch
	truncated := false

	searchOneFile := func(full string) error {
		if fileGlob != "" {
			ok, err := filepath.Match(fileGlob, filepath.Base(full))
			if err != nil {
				return fmt.Errorf("invalid file_glob %q: %w", fileGlob, err)
			}
			if !ok {
				return nil
			}
		}

		f, err := os.Open(full)
		if err != nil {
			return nil // unreadable file (permissions, ...) — skip rather than fail the whole search
		}
		defer f.Close()

		sniff := make([]byte, 8000)
		n, _ := f.Read(sniff)
		if isBinary(sniff[:n]) {
			return nil
		}
		if _, err := f.Seek(0, 0); err != nil {
			return nil
		}

		relToWorkspace, err := filepath.Rel(t.ws.Root(), full)
		if err != nil {
			relToWorkspace = full
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if len(matches) >= maxSearchMatches {
				truncated = true
				return nil
			}
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, searchMatch{
					Path: filepath.ToSlash(relToWorkspace),
					Line: lineNum,
					Text: tools.Truncate(line, maxSearchLineChar),
				})
			}
		}
		return nil
	}

	if !info.IsDir() {
		if err := searchOneFile(resolved); err != nil {
			return failure(err.Error()), nil
		}
	} else {
		err = filepath.Walk(resolved, func(full string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fi.IsDir() {
				if full != resolved && defaultIgnoredDirs[fi.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if truncated {
				return filepath.SkipAll
			}
			return searchOneFile(full)
		})
		if err != nil {
			return failure(fmt.Sprintf("failed to search %s: %v", path, err)), nil
		}
	}

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
