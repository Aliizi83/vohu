package tools_test

import (
	"strings"
	"testing"

	"github.com/Aliizi83/vohu/internal/tools"
)

func TestTruncate_LeavesShortStringsUntouched(t *testing.T) {
	s := "short"
	if got := tools.Truncate(s, 100); got != s {
		t.Fatalf("expected no change, got %q", got)
	}
}

func TestTruncate_KeepsFirstAndLastHalf(t *testing.T) {
	s := strings.Repeat("a", 50) + strings.Repeat("b", 50)
	got := tools.Truncate(s, 40)

	if !strings.HasPrefix(got, strings.Repeat("a", 20)) {
		t.Fatalf("expected the truncated output to start with the original prefix, got %q", got[:40])
	}
	if !strings.HasSuffix(got, strings.Repeat("b", 20)) {
		t.Fatalf("expected the truncated output to end with the original suffix, got %q", got[len(got)-40:])
	}
	if !strings.Contains(got, "omitted") {
		t.Fatalf("expected a note about omitted content, got %q", got)
	}
}

func TestTruncate_ZeroOrNegativeMaxIsANoOp(t *testing.T) {
	s := "some content"
	if got := tools.Truncate(s, 0); got != s {
		t.Fatalf("expected maxChars=0 to be a no-op, got %q", got)
	}
	if got := tools.Truncate(s, -1); got != s {
		t.Fatalf("expected a negative maxChars to be a no-op, got %q", got)
	}
}
