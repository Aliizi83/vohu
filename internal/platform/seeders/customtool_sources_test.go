package seeders_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Aliizi83/vohu/internal/toolbuild"
)

// These sources are embedded via go:embed inside internal/platform/seeders
// (an internal package), so they're re-read directly from disk here rather
// than imported — this test's only job is to prove each one actually
// compiles and behaves correctly under the stdin/stdout JSON contract.
func readSource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("customtool_sources", name))
	if err != nil {
		t.Fatalf("failed to read %s: %v", name, err)
	}
	return string(data)
}

func buildTool(t *testing.T, sourceFile string) string {
	t.Helper()

	builder := toolbuild.NewGoBuilder()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	binary, err := builder.Build(ctx, toolbuild.Request{
		SourceCode: readSource(t, sourceFile), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
	})
	if err != nil {
		t.Fatalf("Build(%s) failed: %v", sourceFile, err)
	}

	path := filepath.Join(t.TempDir(), sourceFile+".bin")
	if err := os.WriteFile(path, binary, 0o700); err != nil {
		t.Fatalf("failed to write binary: %v", err)
	}
	return path
}

func runTool(t *testing.T, binary string, workDir string, input any) map[string]any {
	t.Helper()

	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("running %s failed: %v (stderr: %s)", binary, err, stderr.String())
	}

	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output from %s: %v (raw: %s)", binary, err, stdout.String())
	}
	return result
}

func TestCustomToolSources_FullFileLifecycle(t *testing.T) {
	workDir := t.TempDir()

	writeFile := buildTool(t, "write_file.go.txt")
	readFile := buildTool(t, "read_file.go.txt")
	editFile := buildTool(t, "edit_file.go.txt")
	listDirectory := buildTool(t, "list_directory.go.txt")
	searchFiles := buildTool(t, "search_files.go.txt")
	findFiles := buildTool(t, "find_files.go.txt")

	// write_file: create
	result := runTool(t, writeFile, workDir, map[string]any{"path": "notes.txt", "content": "hello world\nsecond line\n"})
	if result["success"] != true {
		t.Fatalf("write_file (create) failed: %+v", result)
	}
	data := result["data"].(map[string]any)
	if data["action"] != "created" {
		t.Fatalf("expected action \"created\", got %+v", data)
	}

	// read_file
	result = runTool(t, readFile, workDir, map[string]any{"path": "notes.txt"})
	if result["success"] != true {
		t.Fatalf("read_file failed: %+v", result)
	}
	data = result["data"].(map[string]any)
	content := data["content"].(string)
	if !strings.Contains(content, "hello world") || !strings.Contains(content, "second line") {
		t.Fatalf("expected content to contain both lines, got %q", content)
	}

	// write_file: overwrite
	result = runTool(t, writeFile, workDir, map[string]any{"path": "notes.txt", "content": "replaced\n"})
	data = result["data"].(map[string]any)
	if data["action"] != "overwritten" {
		t.Fatalf("expected action \"overwritten\", got %+v", data)
	}

	// edit_file
	result = runTool(t, editFile, workDir, map[string]any{"path": "notes.txt", "old_str": "replaced", "new_str": "edited"})
	if result["success"] != true {
		t.Fatalf("edit_file failed: %+v", result)
	}
	result = runTool(t, readFile, workDir, map[string]any{"path": "notes.txt"})
	data = result["data"].(map[string]any)
	if !strings.Contains(data["content"].(string), "edited") {
		t.Fatalf("expected edited content, got %+v", data["content"])
	}

	// edit_file: old_str not found
	result = runTool(t, editFile, workDir, map[string]any{"path": "notes.txt", "old_str": "nope", "new_str": "x"})
	if result["success"] != false {
		t.Fatalf("expected edit_file to fail for a non-matching old_str, got %+v", result)
	}

	// list_directory
	result = runTool(t, listDirectory, workDir, map[string]any{"path": "."})
	if result["success"] != true {
		t.Fatalf("list_directory failed: %+v", result)
	}
	data = result["data"].(map[string]any)
	entries := data["entries"].([]any)
	found := false
	for _, e := range entries {
		if e.(map[string]any)["path"] == "notes.txt" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected notes.txt in listing, got %+v", entries)
	}

	// search_files
	result = runTool(t, searchFiles, workDir, map[string]any{"pattern": "edited"})
	if result["success"] != true {
		t.Fatalf("search_files failed: %+v", result)
	}
	data = result["data"].(map[string]any)
	if data["count"].(float64) < 1 {
		t.Fatalf("expected at least one match, got %+v", data)
	}

	// find_files
	result = runTool(t, findFiles, workDir, map[string]any{"pattern": "*.txt"})
	if result["success"] != true {
		t.Fatalf("find_files failed: %+v", result)
	}
	data = result["data"].(map[string]any)
	matches := data["matches"].([]any)
	if len(matches) != 1 || matches[0] != "notes.txt" {
		t.Fatalf("expected [\"notes.txt\"], got %+v", matches)
	}
}

func TestCustomToolSources_PathEscapeIsRejected(t *testing.T) {
	workDir := t.TempDir()
	readFile := buildTool(t, "read_file.go.txt")

	result := runTool(t, readFile, workDir, map[string]any{"path": "../../etc/passwd"})
	if result["success"] != false {
		t.Fatalf("expected a path escape to be rejected, got %+v", result)
	}
}

// TestCustomToolSources_AbsolutePathMatchingWorkDirIsNotDoubled guards the
// bug ssh_execute and these tools disagreed on in practice: a caller who
// (very naturally) supplies the real absolute path to a file already
// inside the tool's own working directory — e.g. workDir is an SSH login
// shell's home directory, and the caller writes to that same absolute
// path — must land at that literal path, not workDir+path. Before the
// fix, filepath.Join(workDir, path) doesn't special-case an absolute
// second argument, it just concatenates, silently doubling the path into
// somewhere ssh_execute (which treats paths literally, no sandboxing)
// would never find it.
func TestCustomToolSources_AbsolutePathMatchingWorkDirIsNotDoubled(t *testing.T) {
	workDir := t.TempDir()
	writeFile := buildTool(t, "write_file.go.txt")
	readFile := buildTool(t, "read_file.go.txt")

	absPath := filepath.Join(workDir, "file_manager.py")

	result := runTool(t, writeFile, workDir, map[string]any{"path": absPath, "content": "print('hi')\n"})
	if result["success"] != true {
		t.Fatalf("write_file with an absolute in-workdir path failed: %+v", result)
	}

	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("expected the file at the literal absolute path %q, got: %v", absPath, err)
	}
	doubled := filepath.Join(workDir, absPath)
	if _, err := os.Stat(doubled); err == nil {
		t.Fatalf("file was written to the doubled path %q (workDir joined onto an absolute path) instead of the literal one", doubled)
	}

	result = runTool(t, readFile, workDir, map[string]any{"path": absPath})
	if result["success"] != true {
		t.Fatalf("read_file with the same absolute path failed: %+v", result)
	}
	data := result["data"].(map[string]any)
	if !strings.Contains(data["content"].(string), "print('hi')") {
		t.Fatalf("expected the written content back, got %+v", data["content"])
	}
}
