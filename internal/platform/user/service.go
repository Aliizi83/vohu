package user

import (
	"context"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var ErrUsernameTaken = errors.New("username already taken")

// Service is what other modules (e.g. auth) depend on — never Repository
// directly.
type Service interface {
	Register(ctx context.Context, req CreateUserRequest) (Response, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id uint) (*User, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, req CreateUserRequest) (Response, error) {

	exists, err := s.repo.ExistsByUsername(ctx, req.Username)
	if err != nil {
		return Response{}, err
	}
	if exists {
		return Response{}, ErrUsernameTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return Response{}, err
	}

	u := &User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashed),
		Enabled:  true,
	}

	if err := s.repo.Create(ctx, u); err != nil {
		return Response{}, err
	}

	return toResponse(*u), nil
}

func (s *service) GetByUsername(ctx context.Context, username string) (*User, error) {
	return s.repo.FindByUsername(ctx, username)
}

func (s *service) GetByID(ctx context.Context, id uint) (*User, error) {
	return s.repo.FindByID(ctx, id)
}
