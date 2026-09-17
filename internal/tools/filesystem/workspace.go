// Package filesystem implements the agent's file-editing tools
// (read_file, write_file, edit_file, list_directory) — every one of them
// resolves its path through a shared Workspace so path-traversal
// protection lives in exactly one place instead of being reimplemented
// per tool.
package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace pins every tool-facing path to one root directory. Resolve
// is the only way tools should turn a model-supplied path into a real
// filesystem path — never call filepath.Join on the root directly.
type Workspace struct {
	root string
}

// NewWorkspace resolves root to an absolute, symlink-free path (so later
// containment checks compare like-for-like) and verifies it's an
// existing directory.
func NewWorkspace(root string) (*Workspace, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	info, err := os.Stat(real)
	if err != nil {
		return nil, fmt.Errorf("workspace root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace root %q is not a directory", root)
	}

	return &Workspace{root: real}, nil
}

// Root returns the workspace's resolved absolute root.
func (w *Workspace) Root() string {
	return w.root
}

// Resolve turns a model-supplied path into a real filesystem path
// guaranteed to be inside the workspace, or an error if it would escape.
// A relative path is joined onto root, as expected. An absolute-looking
// path is NOT joined onto root — filepath.Join doesn't special-case an
// absolute second argument, it just concatenates, so joining root onto
// an already-root-prefixed absolute path (e.g. root="/home/developer",
// path="/home/developer/x") would silently double it into
// "/home/developer/home/developer/x" instead of the "/home/developer/x"
// the caller almost certainly meant. An absolute path is instead taken
// literally and only has to pass the same containment check below.
//
// Two escapes are checked, not one: the "../../etc/passwd"-style string
// escape (caught by re-verifying the cleaned path is still prefixed by
// root — filepath.Join+Clean alone does NOT prevent this, it happily
// produces a path outside root if given enough ".." segments) and a
// symlink escape (an existing path inside the workspace whose target —
// or an ancestor directory's target — actually points outside it,
// caught by re-checking containment after EvalSymlinks on whatever part
// of the path exists).
func (w *Workspace) Resolve(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path is required")
	}

	var cleaned string
	if filepath.IsAbs(path) {
		cleaned = filepath.Clean(path)
	} else {
		cleaned = filepath.Clean(filepath.Join(w.root, path))
	}

	if !w.contains(cleaned) {
		return "", fmt.Errorf("path %q escapes the workspace", path)
	}

	real, err := realizeExistingPrefix(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	if !w.contains(real) {
		return "", fmt.Errorf("path %q escapes the workspace (via a symlink)", path)
	}

	return cleaned, nil
}

func (w *Workspace) contains(path string) bool {
	return path == w.root || strings.HasPrefix(path, w.root+string(filepath.Separator))
}

// realizeExistingPrefix resolves symlinks along path, walking up to the
// nearest existing ancestor first — path itself doesn't have to exist
// yet (write_file creates new files), but every existing component's
// real location still has to be checked.
func realizeExistingPrefix(path string) (string, error) {
	if _, err := os.Lstat(path); err == nil {
		return filepath.EvalSymlinks(path)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	parent := filepath.Dir(path)
	if parent == path {
		return path, nil
	}

	realParent, err := realizeExistingPrefix(parent)
	if err != nil {
		return "", err
	}

	return filepath.Join(realParent, filepath.Base(path)), nil
}
