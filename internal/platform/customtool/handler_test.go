package customtool

import "testing"

func TestParseGoErrors_ExtractsMultipleDiagnostics(t *testing.T) {
	raw := "building: go [build -o /tmp/x/tool.bin .]: exit status 1: # customtool\n" +
		"./main.go:4:2: undefined: undefinedFunc\n" +
		"./main.go:5:6: declared and not used: x\n"

	diagnostics := ParseGoErrors(raw)

	if len(diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d: %+v", len(diagnostics), diagnostics)
	}
	if diagnostics[0].Line != 4 || diagnostics[0].Column != 2 || diagnostics[0].Message != "undefined: undefinedFunc" {
		t.Fatalf("unexpected first diagnostic: %+v", diagnostics[0])
	}
	if diagnostics[1].Line != 5 || diagnostics[1].Column != 6 || diagnostics[1].Message != "declared and not used: x" {
		t.Fatalf("unexpected second diagnostic: %+v", diagnostics[1])
	}
}

func TestParseGoErrors_FallsBackToWholeMessageWhenNoLineMatch(t *testing.T) {
	raw := "resolving dependencies: go [mod tidy]: exit status 1: go: finding module for package some/typo"

	diagnostics := ParseGoErrors(raw)

	if len(diagnostics) != 1 || diagnostics[0].Message != raw {
		t.Fatalf("expected a single fallback diagnostic carrying the raw message, got %+v", diagnostics)
	}
}
