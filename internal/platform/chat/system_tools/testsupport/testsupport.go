// Package testsupport holds test doubles shared across the system_tools
// subpackages. It's a regular (non-_test.go) package on purpose: Go never
// compiles one package's _test.go files into another package's test
// binary, so a stub meant to be reused across ssh_tool/custom_tool/
// list_connections has to live somewhere importable like this, not in
// any one of their _test.go files.
package testsupport

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
)

// StubSSHConnService is a minimal sshconn.Service double — only GetByID,
// DecryptPrivateKey, and List are ever reached by the tools under test;
// the rest just panic if a test somehow calls them.
type StubSSHConnService struct {
	Conn      *sshconn.SSHConnection
	GetErr    error
	Secret    string
	SecretErr error
	Items     []sshconn.SSHConnection
}

func (s *StubSSHConnService) Create(context.Context, uint, sshconn.CreateSSHConnectionRequest) (*sshconn.SSHConnection, error) {
	panic("not used by this tool")
}
func (s *StubSSHConnService) GetByID(ctx context.Context, id uint) (*sshconn.SSHConnection, error) {
	if s.GetErr != nil {
		return nil, s.GetErr
	}
	return s.Conn, nil
}
func (s *StubSSHConnService) Update(context.Context, uint, sshconn.UpdateSSHConnectionRequest) (*sshconn.SSHConnection, error) {
	panic("not used by this tool")
}
func (s *StubSSHConnService) Delete(context.Context, uint) error { panic("not used by this tool") }
func (s *StubSSHConnService) List(context.Context, shared.DynamicFilter, shared.Pagination) ([]sshconn.SSHConnection, int64, error) {
	return s.Items, int64(len(s.Items)), nil
}
func (s *StubSSHConnService) ListForCaller(context.Context, uint, shared.DynamicFilter, shared.Pagination) ([]sshconn.SSHConnection, int64, error) {
	panic("not used by this tool")
}
func (s *StubSSHConnService) GetByIDForCaller(context.Context, uint, uint) (*sshconn.SSHConnection, error) {
	panic("not used by this tool")
}
func (s *StubSSHConnService) DecryptPrivateKey(*sshconn.SSHConnection) (string, error) {
	if s.SecretErr != nil {
		return "", s.SecretErr
	}
	return s.Secret, nil
}

func DenyAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return false, nil
}

func AllowAccess(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) (bool, error) {
	return true, nil
}

// NoopCommandRules is a commandrule.Service double returning zero rules
// (deny-everything under accept-mode) for every connection — only
// ListForConnection needs a real implementation; the rest panic if a
// test somehow calls them.
type NoopCommandRules struct{}

func (NoopCommandRules) Create(context.Context, commandrule.CreateRuleRequest) (*commandrule.Rule, error) {
	panic("not used by this tool")
}
func (NoopCommandRules) GetByID(context.Context, uint) (*commandrule.Rule, error) {
	panic("not used by this tool")
}
func (NoopCommandRules) Update(context.Context, uint, commandrule.UpdateRuleRequest) (*commandrule.Rule, error) {
	panic("not used by this tool")
}
func (NoopCommandRules) Delete(context.Context, uint) error { panic("not used by this tool") }
func (NoopCommandRules) ListForConnection(context.Context, uint, shared.Pagination) ([]commandrule.Rule, int64, error) {
	return nil, 0, nil
}
