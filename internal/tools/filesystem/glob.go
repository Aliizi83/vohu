package filesystem

import (
	"path/filepath"
	"strings"
)

// matchGlob reports whether path (slash-separated, relative) matches
// pattern. Beyond filepath.Match's single-segment "*"/"?"/"[...]", a "**"
// path segment matches zero or more whole path segments — the standard
// "doublestar" extension every find_files-style tool needs to express
// something like "**/*.test.go" (any depth). "**" is only special as its
// own path segment; "a**b" within one segment is matched literally by
// filepath.Match, same as any other glob implementation.
func matchGlob(pattern, path string) (bool, error) {
	return matchGlobParts(strings.Split(pattern, "/"), strings.Split(path, "/"))
}

func matchGlobParts(pattern, path []string) (bool, error) {
	if len(pattern) == 0 {
		return len(path) == 0, nil
	}

	if pattern[0] == "**" {
		// Try consuming zero segments with "**" first, then one more,
		// then two more, ... — the standard backtracking approach for
		// matching a wildcard that can span any number of segments.
		if ok, err := matchGlobParts(pattern[1:], path); err != nil || ok {
			return ok, err
		}
		if len(path) == 0 {
			return false, nil
		}
		return matchGlobParts(pattern, path[1:])
	}

	if len(path) == 0 {
		return false, nil
	}

	matched, err := filepath.Match(pattern[0], path[0])
	if err != nil || !matched {
		return false, err
	}

	return matchGlobParts(pattern[1:], path[1:])
}
