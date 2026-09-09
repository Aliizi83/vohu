package sshconn

import (
	"context"
	"errors"
	"fmt"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/crypto"
)

// GrantCreatorAccess is the shape of rbac.Service.GrantResourceAccess,
// injected the same way every cross-module dependency is in this
// codebase — a function value, so this module never imports rbac. Called
// once, right after a connection is created, so its creator isn't locked
// out of the row they just made. level/effect are plain strings
// (rbac.AccessLevel/ResourceEffect's underlying type).
type GrantCreatorAccess func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string, effect string) error

// TestConnectionFunc verifies a private key actually authenticates
// against a host before Service.Create ever writes a row — injected the
// same way every cross-cutting capability this package needs from outside
// is (GrantCreatorAccess, shared.AccessLevelCheck): a function value, so
// this package never imports internal/tools/command directly (only
// chat.Handler does — see its doc comment). The concrete implementation
// (command.TestDial) is wired up in cmd/server/main.go.
type TestConnectionFunc func(ctx context.Context, host string, port int, username string, privateKey string) error

// ErrConnectionTestFailed wraps whatever TestConnectionFunc returned —
// errors.Is(err, ErrConnectionTestFailed) is how the handler recognizes
// "the input was well-formed but the connection doesn't actually work"
// and reports it as a 400 rather than a 500.
var ErrConnectionTestFailed = errors.New("ssh connection test failed")

const ResourceTypeSSHConnection = "ssh_connection"

// Service is what the chat module depends on — never Repository directly.
// Get/Update/Delete need no *ForCaller counterpart the way List does:
// they're gated by shared.RequireAccessLevelOnParam at the route level
// (which already tries the caller's exact grant on this connection, then
// falls back to a wildcard grant, then their roles', before ever reaching
// the handler), so by the time these run the caller is already cleared.
type Service interface {
	// Create tests the connection (dials the host, completes the SSH auth
	// handshake with the given private key) before writing anything — a
	// connection that doesn't actually work is never persisted at all,
	// see ErrConnectionTestFailed.
	Create(ctx context.Context, userID uint, req CreateSSHConnectionRequest) (*SSHConnection, error)
	GetByID(ctx context.Context, id uint) (*SSHConnection, error)
	Update(ctx context.Context, id uint, req UpdateSSHConnectionRequest) (*SSHConnection, error)
	Delete(ctx context.Context, id uint) error
	// List is the raw, unfiltered query — used internally by
	// ListForCaller's wildcard-access fast path, and by chat's SSHTool/
	// ListSSHConnectionsTool, which already do their own explicit
	// HasAccessLevel check per row independent of this.
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)

	// ListForCaller shows a caller holding wildcard "read" every
	// connection (unchanged admin behavior); anyone else only the
	// connections they hold at least Read-level resource access to.
	ListForCaller(ctx context.Context, userID uint, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)

	// DecryptPrivateKey returns a connection's plaintext private key —
	// only ever called server-side, right before dialing, never exposed
	// through any HTTP response.
	DecryptPrivateKey(conn *SSHConnection) (string, error)
}

type service struct {
	repo           Repository
	box            *crypto.Box
	grantAccess    GrantCreatorAccess
	hasAccessLevel shared.AccessLevelCheck
	testConnection TestConnectionFunc
}

func NewService(
	repo Repository,
	box *crypto.Box,
	grantAccess GrantCreatorAccess,
	hasAccessLevel shared.AccessLevelCheck,
	testConnection TestConnectionFunc,
) Service {
	return &service{
		repo:           repo,
		box:            box,
		grantAccess:    grantAccess,
		hasAccessLevel: hasAccessLevel,
		testConnection: testConnection,
	}
}

func (s *service) Create(ctx context.Context, userID uint, req CreateSSHConnectionRequest) (*SSHConnection, error) {
	port := req.Port
	if port == 0 {
		port = 22
	}

	if err := s.testConnection(ctx, req.Host, port, req.Username, req.PrivateKey); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectionTestFailed, err)
	}

	encrypted, err := s.box.Encrypt(req.PrivateKey)
	if err != nil {
		return nil, err
	}

	conn := &SSHConnection{
		Name:                req.Name,
		Host:                req.Host,
		Port:                port,
		Username:            req.Username,
		EncryptedPrivateKey: encrypted,
		CreatedByUserID:     userID,
	}

	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, err
	}

	// The creator gets full (manage) access to their own connection —
	// otherwise nobody could use a connection they just made, since
	// resource access is default-deny. "manage" rather than a lower level
	// so the creator can also grant others access to it later.
	if err := s.grantAccess(ctx, userID, ResourceTypeSSHConnection, conn.ID, "manage", "accepted"); err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *service) GetByID(ctx context.Context, id uint) (*SSHConnection, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Update(ctx context.Context, id uint, req UpdateSSHConnectionRequest) (*SSHConnection, error) {
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != "" {
		conn.Name = req.Name
	}
	if req.Host != "" {
		conn.Host = req.Host
	}
	if req.Port != 0 {
		conn.Port = req.Port
	}
	if req.Username != "" {
		conn.Username = req.Username
	}
	if req.PrivateKey != "" {
		encrypted, err := s.box.Encrypt(req.PrivateKey)
		if err != nil {
			return nil, err
		}
		conn.EncryptedPrivateKey = encrypted
	}

	if err := s.repo.Update(ctx, conn); err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) List(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]SSHConnection, int64, error) {
	return s.repo.List(ctx, filter, page)
}

func (s *service) DecryptPrivateKey(conn *SSHConnection) (string, error) {
	return s.box.Decrypt(conn.EncryptedPrivateKey)
}

func (s *service) ListForCaller(
	ctx context.Context,
	userID uint,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]SSHConnection, int64, error) {
	canSeeAll, err := s.hasAccessLevel(ctx, userID, ResourceTypeSSHConnection, shared.WildcardResourceID, "read")
	if err != nil {
		return nil, 0, err
	}
	if canSeeAll {
		return s.repo.List(ctx, filter, page)
	}

	candidates, _, err := s.repo.List(ctx, filter, shared.Pagination{PageNumber: 1, PageSize: 1000})
	if err != nil {
		return nil, 0, err
	}

	return shared.FilterAndPaginate(
		ctx, candidates, func(c SSHConnection) uint { return c.ID },
		s.hasAccessLevel, userID, ResourceTypeSSHConnection, "read", page,
	)
}
