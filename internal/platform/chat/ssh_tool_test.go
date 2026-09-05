package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools/command"
)

// stubSSHConnService is a minimal sshconn.Service double — only GetByID
// and DecryptSecret are ever reached by SSHTool.Execute, so the rest just
// panic if a test somehow calls them.
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
func (s *stubSSHConnService) DecryptSecret(*sshconn.SSHConnection) (string, error) {
	if s.secretErr != nil {
		return "", s.secretErr
	}
	return s.secret, nil
}

func denyAccess(ctx context.Context, userID uint, resourceType string, resourceID uint) (bool, error) {
	return false, nil
}

func allowAccess(ctx context.Context, userID uint, resourceType string, resourceID uint) (bool, error) {
	return true, nil
}

func TestSSHTool_Execute_MissingConnectionID(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopPolicy{})

	result, err := tool.Execute(context.Background(), map[string]any{"program": "ls"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when connectionId is missing")
	}
}

func TestSSHTool_Execute_MissingProgram(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopPolicy{})

	result, err := tool.Execute(context.Background(), map[string]any{"connectionId": float64(5)})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure when program is missing")
	}
}

func TestSSHTool_Execute_DeniedAccessNeverReachesConnectionLookup(t *testing.T) {
	svc := &stubSSHConnService{getErr: errors.New("GetByID should never be called")}
	tool := NewSSHTool(1, svc, denyAccess, noopPolicy{})

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
	tool := NewSSHTool(1, svc, allowAccess, noopPolicy{})

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

func TestSSHTool_Execute_UnknownAuthMethod(t *testing.T) {
	svc := &stubSSHConnService{
		conn:   &sshconn.SSHConnection{Host: "example.com", Port: 22, Username: "u", AuthMethod: "carrier_pigeon"},
		secret: "irrelevant",
	}
	tool := NewSSHTool(1, svc, allowAccess, noopPolicy{})

	result, err := tool.Execute(context.Background(), map[string]any{
		"connectionId": float64(5),
		"program":      "ls",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Success {
		t.Fatal("expected failure for an unrecognized auth method")
	}
}

func TestSSHTool_Execute_ArgsMustBeStringArray(t *testing.T) {
	tool := NewSSHTool(1, &stubSSHConnService{}, allowAccess, noopPolicy{})

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

// noopPolicy allows nothing — SSHExecutor.Execute short-circuits on
// policy denial before any network I/O, which is all these tests need:
// SSHTool.Execute is expected to fail earlier still (missing args,
// permission denial, unknown auth method) before ever reaching the
// executor.
type noopPolicy struct{}

func (noopPolicy) Evaluate(command.Command) command.Decision {
	return command.Decision{Allowed: false, Reason: "test policy denies everything"}
}
