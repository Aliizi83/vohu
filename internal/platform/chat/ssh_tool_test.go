package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
)

// stubSSHConnService is a minimal sshconn.Service double — only GetByID
// and DecryptPrivateKey are ever reached by SSHTool.Execute, so the rest
// just panic if a test somehow calls them.
type stubSSHConnService struct {
	conn      *sshconn.SSHConnection
	getErr    error
	secret    string
	secretErr error
}

func (s *stubSSHConnService) Create(context.Context, uint, sshconn.CreateSSHConnectionRequest) (*sshconn.SSHConnection, error) {
	panic("not used by SSHTool")
}
func (s *stubSSHConnService) GetByID(ctx context.Context, id uint) (*sshconn.SSHConnection, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.conn, nil
}
func (s *stubSSHConnService) Update(context.Context, uint, sshconn.UpdateSSHConnectionRequest) (*sshconn.SSHConnection, error) {
	panic("not used by SSHTool")
}
func (s *stubSSHConnService) Delete(context.Context, uint) error { panic("not used by SSHTool") }
func (s *stubSSHConnService) List(context.Context, shared.DynamicFilter, shared.Pagination) ([]sshconn.SSHConnection, int64, error) {
	panic("not used by SSHTool")
}
func (s *stubSSHConnService) ListForCaller(context.Context, uint, shared.DynamicFilter, shared.Pagination) ([]sshconn.SSHConnection, int64, error) {
	panic("not used by SSHTool")
}
func (s *stubSSHConnService) GetByIDForCaller(context.Context, uint, uint) (*sshconn.SSHConnection, error) {
	panic("not used by SSHTool")
}
func (s *stubSSHConnService) DecryptPrivateKey(*sshconn.SSHConnection) (string, error) {
	if s.secretErr != nil {
		return "", s.secretErr
	}
	return s.secret, nil
}

func denyAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func allowAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

func TestSSHTool_Execute_MissingConnectionID(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{"program": "ls"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when connectionId is missing")
	}
}

func TestSSHTool_Execute_MissingProgram(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(5)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when program is missing")
	}
}

func TestSSHTool_Execute_RequestsWriteLevel(t *testing.T) {
	var gotLevel string
	spy := func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
		gotLevel = level
		return false, nil // deny is fine — this test only cares which level was requested
	}

	tool := NewSSHTool(1, &stubSSHConnService{}, spy, noopCommandRules{})
	_, _ = tool.Execute(context.Background(), map[string]any{"connectionId": float64(5), "program": "ls"})

	if gotLevel != "write" {
		t.Fatalf("expected SSHTool to request level %q, got %q", "write", gotLevel)
	}
}

func TestSSHTool_Execute_DeniedAccessNeverReachesConnectionLookup(t *testing.T) {
	svc := &stubSSHConnService{getErr: errors.New("GetByID should never be called")}
	tool := NewSSHTool(1, svc, denyAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when CanAccessResource denies")
	}
	if data, ok := result.Data.(string); !ok || data == "" {
		t.Fatalf("expected a non-empty denial reason, got %+v", result.Data)
	}
}

func TestSSHTool_Execute_ConnectionNotFound(t *testing.T) {
	svc := &stubSSHConnService{getErr: shared.ErrNotFound}
	tool := NewSSHTool(1, svc, allowAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when the connection doesn't exist")
	}
}

func TestSSHTool_Execute_UnparseablePrivateKey(t *testing.T) {
	svc := &stubSSHConnService{
		conn:   &sshconn.SSHConnection{Host: "example.com", Port: 22, Username: "u"},
		secret: "not a real private key",
	}
	tool := NewSSHTool(1, svc, allowAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for an unparseable private key")
	}
}

func TestSSHTool_Execute_ArgsMustBeStringArray(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopCommandRules{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
		"args":         "not-an-array",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when args isn't an array")
	}
}

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
			got, ok := parseUintArg(tc.in)
			if ok != tc.wantOK || got != tc.want {
				t.Fatalf("parseUintArg(%v) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseStringArrayArg(t *testing.T) {
	got, err := parseStringArrayArg([]any{"status", "-s"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "status" || got[1] != "-s" {
		t.Fatalf("unexpected result: %v", got)
	}

	if _, err := parseStringArrayArg([]any{"ok", 5}); err == nil {
		t.Fatal("expected an error when an array element isn't a string")
	}

	if _, err := parseStringArrayArg("not-an-array"); err == nil {
		t.Fatal("expected an error when the value isn't an array")
	}

	got, err = parseStringArrayArg(nil)
	if err != nil || got != nil {
		t.Fatalf("expected (nil, nil) for a nil value, got (%v, %v)", got, err)
	}
}

// noopCommandRules is a commandrule.Service double returning zero rules
// (deny-everything under accept-mode) for every connection — none of the
// tests above ever reach the point where SSHTool.Execute would actually
// read it (they all fail earlier: missing args, permission denial,
// unknown connection, unparseable key), so only ListForConnection needs a
// real implementation; the rest panic if a test somehow calls them.
type noopCommandRules struct{}

func (noopCommandRules) Create(context.Context, commandrule.CreateRuleRequest) (*commandrule.Rule, error) {
	panic("not used by SSHTool")
}
func (noopCommandRules) GetByID(context.Context, uint) (*commandrule.Rule, error) {
	panic("not used by SSHTool")
}
func (noopCommandRules) Update(context.Context, uint, commandrule.UpdateRuleRequest) (*commandrule.Rule, error) {
	panic("not used by SSHTool")
}
func (noopCommandRules) Delete(context.Context, uint) error { panic("not used by SSHTool") }
func (noopCommandRules) ListForConnection(context.Context, uint, shared.Pagination) ([]commandrule.Rule, int64, error) {
	return nil, 0, nil
}
