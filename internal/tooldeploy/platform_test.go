package tooldeploy

import "testing"

func TestMapGOOS(t *testing.T) {
	cases := map[string]string{"Linux": "linux", "Darwin": "darwin", "linux": "linux"}
	for in, want := range cases {
		if got := mapGOOS(in); got != want {
			t.Errorf("mapGOOS(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMapGOARCH(t *testing.T) {
	cases := map[string]string{
		"x86_64": "amd64", "aarch64": "arm64", "arm64": "arm64",
		"i686": "386", "armv7l": "arm",
	}
	for in, want := range cases {
		if got := mapGOARCH(in); got != want {
			t.Errorf("mapGOARCH(%q) = %q, want %q", in, got, want)
		}
	}
}
