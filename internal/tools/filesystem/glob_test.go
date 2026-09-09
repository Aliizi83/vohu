package filesystem

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "sub/main.go", false}, // no ** — single segment only
		{"**/*.go", "main.go", true},   // ** also matches zero segments
		{"**/*.go", "sub/main.go", true},
		{"**/*.go", "a/b/c/main.go", true},
		{"**/*.test.go", "internal/tools/filesystem/read_file_test.go", false},
		{"**/*_test.go", "internal/tools/filesystem/read_file_test.go", true},
		{"src/**/*.js", "src/index.js", true},
		{"src/**/*.js", "src/a/b/index.js", true},
		{"src/**/*.js", "other/index.js", false},
		{"**", "anything/at/all.txt", true},
		{"*.go", "main.py", false},
	}

	for _, c := range cases {
		got, err := matchGlob(c.pattern, c.path)
		if err != nil {
			t.Fatalf("matchGlob(%q, %q) returned an error: %v", c.pattern, c.path, err)
		}
		if got != c.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
