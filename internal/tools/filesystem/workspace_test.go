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

func TestWorkspace_Resolve_AbsolutePathOutsideRootIsRejected(t *testing.T) {
	ws, err := filesystem.NewWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// An absolute path outside the workspace is rejected outright, not
	// silently joined onto root — joining it (root + "/etc/passwd") would
	// produce a harmless-but-confusing path nobody asked for instead of
	// a clear error.
	if _, err := ws.Resolve("/etc/passwd"); err == nil {
		t.Fatal("expected an absolute path outside the workspace to be rejected")
	}
}

func TestWorkspace_Resolve_AbsolutePathInsideRootResolvesLiterally(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	ws, err := filesystem.NewWorkspace(root)
	if err != nil {
		t.Fatalf("NewWorkspace failed: %v", err)
	}

	// The natural case this guards against: root IS the caller's own
	// home directory (e.g. an SSH login shell's cwd), and the caller
	// supplies what it believes is a real absolute path already inside
	// it. Before this fix, filepath.Join(root, path) doesn't special-case
	// an absolute second argument, it just concatenates — root got
	// silently doubled into the result instead of the literal path
	// resolving as-is.
	resolved, err := ws.Resolve(filepath.Join(root, "a.txt"))
	if err != nil {
		t.Fatalf("expected an absolute path already inside root to resolve, got %v", err)
	}
	if resolved != filepath.Join(root, "a.txt") {
		t.Fatalf("expected the literal path %q, got %q", filepath.Join(root, "a.txt"), resolved)
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
