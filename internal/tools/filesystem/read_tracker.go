package filesystem

import "sync"

// ReadTracker remembers which resolved paths have been read via
// ReadFileTool, so WriteFileTool can refuse to blindly overwrite a file
// the agent never looked at first — the same guard most established
// coding agents use to prevent an accidental clobber. Safe for concurrent
// use; share one instance between a ReadFileTool and a WriteFileTool at
// whatever scope the caller wants the guard to apply (typically one per
// conversation).
type ReadTracker struct {
	mu   sync.Mutex
	read map[string]bool
}

func NewReadTracker() *ReadTracker {
	return &ReadTracker{read: make(map[string]bool)}
}

func (t *ReadTracker) MarkRead(resolvedPath string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.read[resolvedPath] = true
}

func (t *ReadTracker) WasRead(resolvedPath string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.read[resolvedPath]
}
