package filesystem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools/filesystem"
)

func TestWorkspace_Resolve_AllowsPathsInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "sub", "dir"), 0o755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	if _, err := ws.Resolve("a.txt"); err != nil {
		t.Fatalf("expected a.txt to resolve, got %v", err)
	}
	if _, err := ws.Resolve("sub/dir"); err != nil {
		t.Fatalf("expected sub/dir to resolve, got %v", err)
	}
	if _, err := ws.Resolve("does/not/exist/yet.txt"); err != nil {
		t.Fatalf("expected a nonexistent-but-inside path to resolve (for write_file), got %v", err)
	}
}

func TestWorkspace_Resolve_RejectsDotDotEscape(t *testing.T) {
	root := t.TempDir()
	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	for _, escape := range []string{
		"../outside.txt",
		"../../../../../../etc/passwd",
		"sub/../../outside.txt",
	} {
		if _, err := ws.Resolve(escape); err == nil {
			t.Fatalf("expected %q to be rejected as an escape, got no error", escape)
		}
	}
}

func TestWorkspace_Resolve_ReRootsAnAbsoluteLookingPathRatherThanEscaping(t *testing.T) {
	root := t.TempDir()
	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// An absolute path is still rooted at the workspace, not treated as
	// an override — filepath.Join never lets a later absolute-looking
	// component reset to "/".
	resolved, err := ws.Resolve("/etc/passwd")
	if err != nil {
		t.Fatalf("expected /etc/passwd to resolve *inside* the workspace, got error: %v", err)
	}
	if filepath.Dir(resolved) != filepath.Join(root, "etc") {
		t.Fatalf("expected the absolute-looking path to land inside the workspace root, got %q", resolved)
	}
}

func TestWorkspace_Resolve_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	if _, err := ws.Resolve("escape/secret.txt"); err == nil {
		t.Fatal("expected a symlink pointing outside the workspace to be rejected")
	}
}

func TestWorkspace_Resolve_RejectsEmptyPath(t *testing.T) {
	ws, err := filesystem.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	if _, err := ws.Resolve(""); err == nil {
		t.Fatal("expected an empty path to be rejected")
	}
}
