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
// out of the row they just made. level is a plain string
// (rbac.AccessLevel's underlying type).
type GrantCreatorAccess func(ctx context.Context, userID uint, resourceType string, resourceID uint, level string) error

const ResourceTypeSSHConnection = "ssh_connection"

// Service is what the chat module depends on — never Repository directly.
type Service interface {
	Create(ctx context.Context, userID uint, req CreateSSHConnectionRequest) (*SSHConnection, error)
	// GetByID is the raw, unchecked lookup — used internally by chat's
	// SSHTool, which has always done its own explicit HasAccessLevel
	// check (at Write) before ever calling this, independent of anyone's
	// flat ssh:read permission. GetByIDForCaller below is its
	// admin-facing counterpart.
	GetByID(ctx context.Context, id uint) (*SSHConnection, error)
	Update(ctx context.Context, id uint, req UpdateSSHConnectionRequest) (*SSHConnection, error)
	Delete(ctx context.Context, id uint) error
	// List is the raw, unfiltered query — same "chat already checked
	// access itself" reasoning as GetByID.
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)

	// ListForCaller/GetByIDForCaller back the admin HTTP endpoints: a
	// caller holding the flat "ssh:read" permission sees every
	// connection unfiltered (that permission today only exists on the
	// admin role, so this is the existing "admins manage everything"
	// behavior, unchanged); anyone else sees only rows they hold at
	// least Read-level resource access to — filtered at the query level
	// (see Repository.ListAccessibleToUser), not by fetching everything
	// and checking each row in Go, and a row outside their access
	// resolves to shared.ErrNotFound rather than 403 (existence isn't
	// leaked to a caller with no access to it).
	ListForCaller(ctx context.Context, userID uint, filter shared.DynamicFilter, page shared.Pagination) ([]SSHConnection, int64, error)
	GetByIDForCaller(ctx context.Context, userID uint, id uint) (*SSHConnection, error)

	// DecryptSecret returns a connection's plaintext password/private key
	// — only ever called server-side, right before dialing, never
	// exposed through any HTTP response.
	DecryptSecret(conn *SSHConnection) (string, error)
}

type service struct {
	repo           Repository
	box            *crypto.Box
	grantAccess    GrantCreatorAccess
	hasPermission  shared.PermissionCheck
	hasAccessLevel shared.AccessLevelCheck
}

func NewService(
	repo Repository,
	box *crypto.Box,
	grantAccess GrantCreatorAccess,
	hasPermission shared.PermissionCheck,
	hasAccessLevel shared.AccessLevelCheck,
) Service {
	return &service{
		repo:           repo,
		box:            box,
		grantAccess:    grantAccess,
		hasPermission:  hasPermission,
		hasAccessLevel: hasAccessLevel,
	}
}

const flatReadPermission = "ssh:read"

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

	// The creator gets full (manage) access to their own connection —
	// otherwise nobody could use a connection they just made, since
	// resource access is default-deny. "manage" rather than a lower level
	// so the creator can also grant others access to it later.
	if err := s.grantAccess(ctx, userID, ResourceTypeSSHConnection, conn.ID, "manage"); err != nil {
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

func (s *service) ListForCaller(
	ctx context.Context,
	userID uint,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]SSHConnection, int64, error) {
	canSeeAll, err := s.hasPermission(ctx, userID, flatReadPermission)
	if err != nil {
		return nil, 0, err
	}
	if canSeeAll {
		return s.repo.List(ctx, filter, page)
	}

	return s.repo.ListAccessibleToUser(ctx, filter, page, userID)
}

func (s *service) GetByIDForCaller(ctx context.Context, userID uint, id uint) (*SSHConnection, error) {
	conn, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	canSeeAll, err := s.hasPermission(ctx, userID, flatReadPermission)
	if err != nil {
		return nil, err
	}
	if canSeeAll {
		return conn, nil
	}

	allowed, err := s.hasAccessLevel(ctx, userID, ResourceTypeSSHConnection, id, "read")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, shared.ErrNotFound
	}

	return conn, nil
}
