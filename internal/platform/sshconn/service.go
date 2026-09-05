package sshconn

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/pkg/crypto"
)

// GrantCreatorAccess is the shape of rbac.Service.GrantResourceAccess,
// injected the same way every cross-module dependency is in this
// codebase — a function value, so this module never imports rbac. Called
// once, right after a connection is created, so its creator isn't locked
// out of the row they just made.
type GrantCreatorAccess func(ctx context.Context, userID uint, resourceType string, resourceID uint, effect string) error

const ResourceTypeSSHConnection = "ssh_connection"

// Service is what the chat module (later) depends on — never Repository
// directly.
type Service interface {
	Create(ctx context.Context, userID uint, req CreateSSHConnectionRequest) (*SSHConnection, error)
	GetByID(ctx context.Context, id uint) (*SSHConnection, error)
	Update(ctx context.Context, id uint, req UpdateSSHConnectionRequest) (*SSHConnection, error)
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)

	// DecryptSecret returns a connection's plaintext password/private key
	// — only ever called server-side, right before dialing, never
	// exposed through any HTTP response.
	DecryptSecret(conn *SSHConnection) (string, error)
}

type service struct {
	repo        Repository
	box         *crypto.Box
	grantAccess GrantCreatorAccess
}

func NewService(repo Repository, box *crypto.Box, grantAccess GrantCreatorAccess) Service {
	return &service{repo: repo, box: box, grantAccess: grantAccess}
}

func (s *service) Create(ctx context.Context, userID uint, req CreateSSHConnectionRequest) (*SSHConnection, error) {
	encrypted, err := s.box.Encrypt(req.Secret)
	if err != nil {
		return nil, err
	}

	port := req.Port
	if port == 0 {
		port = 22
	}

	conn := &SSHConnection{
		Name:            req.Name,
		Host:            req.Host,
		Port:            port,
		Username:        req.Username,
		AuthMethod:      req.AuthMethod,
		EncryptedSecret: encrypted,
		CreatedByUserID: userID,
	}

	if err := s.repo.Create(ctx, conn); err != nil {
		return nil, err
	}

	// The creator can always reach their own connection — otherwise
	// nobody could use a connection they just made, since resource
	// access is default-deny.
	if err := s.grantAccess(ctx, userID, ResourceTypeSSHConnection, conn.ID, "accepted"); err != nil {
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
	if req.AuthMethod != "" {
		conn.AuthMethod = req.AuthMethod
	}
	if req.Secret != "" {
		encrypted, err := s.box.Encrypt(req.Secret)
		if err != nil {
			return nil, err
		}
		conn.EncryptedSecret = encrypted
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

func (s *service) DecryptSecret(conn *SSHConnection) (string, error) {
	return s.box.Decrypt(conn.EncryptedSecret)
}
