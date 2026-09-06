package user

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameTaken = errors.New("username already taken")

const resourceTypeUser = "user"

// Service is what other modules (e.g. auth) depend on — never Repository
// directly.
type Service interface {
	Register(ctx context.Context, req CreateUserRequest) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id uint) (*User, error)
	Update(ctx context.Context, id uint, req UpdateUserRequest) (*User, error)
	Delete(ctx context.Context, id uint) error
	// List is the raw, unfiltered query — used internally by
	// ListForCaller's wildcard-access fast path and by anything that
	// already knows it's allowed to see everyone (e.g. auth).
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]User, int64, error)

	// ListForCaller shows a caller holding wildcard "read" on "user"
	// every user (unchanged admin behavior); anyone else only the users
	// they hold at least Read-level resource access to — filtered via
	// shared.FilterAndPaginate rather than a fetch-everything-then-check
	// pass over an unbounded table (see that function's doc comment for
	// why this is a per-row check rather than a single SQL query at this
	// project's current scale).
	ListForCaller(ctx context.Context, userID uint, filter shared.DynamicFilter, page shared.Pagination) ([]User, int64, error)
}

type service struct {
	repo           Repository
	hasAccessLevel shared.AccessLevelCheck
}

func NewService(repo Repository, hasAccessLevel shared.AccessLevelCheck) Service {
	return &service{repo: repo, hasAccessLevel: hasAccessLevel}
}

func (s *service) Register(ctx context.Context, req CreateUserRequest) (*User, error) {

	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUsernameTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	u := &User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashed),
		Enabled:  true,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *service) GetByUsername(ctx context.Context, username string) (*User, error) {
	return s.repo.FindByUsername(ctx, username)
}

func (s *service) GetByID(ctx context.Context, id uint) (*User, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *service) Update(ctx context.Context, id uint, req UpdateUserRequest) (*User, error) {
	u, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Email != "" {
		u.Email = req.Email
	}
	if req.Enabled != nil {
		u.Enabled = *req.Enabled
	}

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *service) Delete(ctx context.Context, id uint) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) List(
	ctx context.Context,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]User, int64, error) {
	return s.repo.List(ctx, filter, page)
}

func (s *service) ListForCaller(
	ctx context.Context,
	userID uint,
	filter shared.DynamicFilter,
	page shared.Pagination,
) ([]User, int64, error) {
	canSeeAll, err := s.hasAccessLevel(ctx, userID, resourceTypeUser, shared.WildcardResourceID, "read")
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
		ctx, candidates, func(u User) uint { return u.ID },
		s.hasAccessLevel, userID, resourceTypeUser, "read", page,
	)
}
