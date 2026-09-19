package chat

import (
	"strings"
	"testing"
)

func TestDeriveTitle_CollapsesWhitespaceAndNewlines(t *testing.T) {
	got := deriveTitle("install nginx\non the prod server\n\nplease")
	if got != "install nginx on the prod server please" {
		t.Fatalf("unexpected title: %q", got)
	}
}

func TestDeriveTitle_EmptyOrWhitespaceOnlyFallsBackToDefault(t *testing.T) {
	for _, in := range []string{"", "   ", "\n\t "} {
		if got := deriveTitle(in); got != "New chat" {
			t.Fatalf("deriveTitle(%q) = %q, want \"New chat\"", in, got)
		}
	}
}

func TestDeriveTitle_TruncatesLongContentWithEllipsis(t *testing.T) {
	long := strings.Repeat("a", maxDerivedTitleLength+50)

	got := deriveTitle(long)

	runes := []rune(got)
	if len(runes) != maxDerivedTitleLength+1 { // +1 for the ellipsis rune
		t.Fatalf("expected truncated length %d, got %d (%q)", maxDerivedTitleLength+1, len(runes), got)
	}
	if runes[len(runes)-1] != '…' {
		t.Fatalf("expected the title to end with an ellipsis, got %q", got)
	}
}

func TestDeriveTitle_ShortContentIsUnchanged(t *testing.T) {
	if got := deriveTitle("سلام، یه کار ساده دارم"); got != "سلام، یه کار ساده دارم" {
		t.Fatalf("unexpected title: %q", got)
	}
}
