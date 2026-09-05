package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Aliizi83/vohu/config"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/user"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

// Service depends only on user.Service (an interface), never on
// user.Repository or the User entity's internals — same dependency rule
// as every other module.
type Service interface {
	Login(ctx context.Context, username, password string) (TokenPair, error)
	Refresh(ctx context.Context, refreshToken string) (TokenPair, error)

	// Authentication builds Gin middleware that verifies the bearer token
	// and sets shared.UserIDContextKey for downstream handlers/middleware
	// (e.g. rbac's RequirePermission) to read.
	Authentication() gin.HandlerFunc
}

type service struct {
	cfg         *config.Config
	userService user.Service
}

func NewService(cfg *config.Config, userService user.Service) Service {
	return &service{cfg: cfg, userService: userService}
}

func (s *service) Login(ctx context.Context, username, password string) (TokenPair, error) {
	u, err := s.userService.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.generateTokenPair(u.ID)
}

func (s *service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, err := parseRefreshToken(refreshToken, s.cfg.JWT.RefreshSecret)
	if err != nil {
		return TokenPair{}, ErrInvalidToken
	}

	if _, err := s.userService.GetByID(ctx, claims.UserID); err != nil {
		return TokenPair{}, ErrInvalidToken
	}

	return s.generateTokenPair(claims.UserID)
}

func (s *service) generateTokenPair(userID uint) (TokenPair, error) {
	now := time.Now()

	accessExpiresAt := now.Add(s.cfg.JWT.AccessTokenExpireDuration * time.Minute)
	accessToken, err := signToken(AccessClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}, s.cfg.JWT.Secret)
	if err != nil {
		return TokenPair{}, err
	}

	refreshExpiresAt := now.Add(s.cfg.JWT.RefreshTokenExpireDuration * time.Minute)
	refreshToken, err := signToken(RefreshClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}, s.cfg.JWT.RefreshSecret)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

func (s *service) Authentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		claims, err := parseAccessToken(parts[1], s.cfg.JWT.Secret)
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(shared.UserIDContextKey, claims.UserID)
		c.Next()
	}
}
