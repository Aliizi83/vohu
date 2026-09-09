package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/tools"
)

// maxReadChars caps how much of a file's formatted content one read_file
// call can hand back — independent of offset/limit, which are how the
// model should page through something bigger than this on purpose.
const maxReadChars = 30_000

type ReadFileTool struct {
	ws      *Workspace
	tracker *ReadTracker
}

// NewReadFileTool takes an optional tracker (nil is fine) — when set, a
// successful read marks the path as read so WriteFileTool's
// read-before-overwrite guard is satisfied. Pass the same tracker to
// both tools to wire that up; pass nil to skip the guard entirely.
func NewReadFileTool(ws *Workspace, tracker *ReadTracker) *ReadFileTool {
	return &ReadFileTool{ws: ws, tracker: tracker}
}

func (t *ReadFileTool) Name() string { return "read_file" }

func (t *ReadFileTool) Description() string {
	return "Read the contents of a file, with line numbers. Use offset/limit to page through a large file instead of reading it all at once."
}

func (t *ReadFileTool) Parameters() ai_model.ToolParameters {
	return ai_model.ToolParameters{
		Properties: map[string]ai_model.ToolProperty{
			"path": {
				Type:        "string",
				Description: "Path to the file, relative to the workspace root.",
			},
			"offset": {
				Type:        "integer",
				Description: "1-based line number to start reading from. Omit to start at line 1.",
			},
			"limit": {
				Type:        "integer",
				Description: "Maximum number of lines to return. Omit to read to the end of the file (subject to the output size cap).",
			},
		},
		Required: []string{"path"},
	}
}

func (t *ReadFileTool) Execute(ctx context.Context, args map[string]any) (tools.ToolResult, error) {
	path, err := requiredString(args, "path")
	if err != nil {
		return failure(err.Error()), nil
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

	if isBinary(data) {
		return tools.ToolResult{
			Success: true,
			Data: map[string]any{
				"path":    path,
				"binary":  true,
				"message": fmt.Sprintf("binary file (%d bytes) — contents not shown", len(data)),
			},
		}, nil
	}

	offset, hasOffset, err := optionalInt(args, "offset")
	if err != nil {
		return failure(err.Error()), nil
	}
	limit, hasLimit, err := optionalInt(args, "limit")
	if err != nil {
		return failure(err.Error()), nil
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	start := 0
	if hasOffset {
		if offset < 1 {
			return failure("offset must be 1 or greater"), nil
		}
		start = offset - 1
	}
	if start > totalLines {
		start = totalLines
	}

	end := totalLines
	if hasLimit {
		if limit < 1 {
			return failure("limit must be 1 or greater"), nil
		}
		if start+limit < end {
			end = start + limit
		}
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&b, "%6d\t%s\n", i+1, lines[i])
	}
	content := tools.Truncate(b.String(), maxReadChars)

	if t.tracker != nil {
		t.tracker.MarkRead(resolved)
	}

	return tools.ToolResult{
		Success: true,
		Data: map[string]any{
			"path":       path,
			"content":    content,
			"totalLines": totalLines,
			"startLine":  start + 1,
			"endLine":    end,
		},
	}, nil
}

// isBinary uses the same practical heuristic tools like grep/git use: a
// NUL byte anywhere in the first chunk means "not text."
func isBinary(data []byte) bool {
	const sniffSize = 8000
	if len(data) > sniffSize {
		data = data[:sniffSize]
	}
	return bytes.IndexByte(data, 0) != -1
}

func failure(message string) tools.ToolResult {
	return tools.ToolResult{Success: false, Data: message}
}
