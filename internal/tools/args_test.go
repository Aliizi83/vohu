package tools

import "testing"

func TestParseUintArg(t *testing.T) {
	cases := []struct {
		name   string
		in     any
		want   uint
		wantOK bool
	}{
		{"float64", float64(42), 42, true},
		{"int", 7, 7, true},
		{"negative float64", float64(-1), 0, false},
		{"string", "42", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseUintArg(tc.in)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("ParseUintArg(%v) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseStringArrayArg(t *testing.T) {
	got, err := ParseStringArrayArg([]any{"status", "-s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "status" || got[1] != "-s" {
		t.Fatalf("unexpected result: %v", got)
	}

	if _, err := ParseStringArrayArg([]any{"ok", 5}); err == nil {
		t.Fatal("expected an error when an array element isn't a string")
	}

	if _, err := ParseStringArrayArg("not-an-array"); err == nil {
		t.Fatal("expected an error when the value isn't an array")
	}

	got, err = ParseStringArrayArg(nil)
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil) for a nil value, got (%v, %v)", got, err)
	}
}
