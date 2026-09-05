package auth

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

// AccessClaims / RefreshClaims: only golang-jwt/jwt/v5 is used anywhere in
// this module (sample-golang-project mixed v3 and v5, which is why its
// middleware did an unchecked `err.(*jwt.ValidationError)` type assertion —
// that type doesn't even exist the same way in v5). v5 gives typed
// sentinel errors (jwt.ErrTokenExpired, etc.) checked with errors.Is
// instead, so there's no panic risk here.

type AccessClaims struct {
	UserID uint `json:"userId"`
	jwt.RegisteredClaims
}

type RefreshClaims struct {
	UserID uint `json:"userId"`
	jwt.RegisteredClaims
}

func signToken(claims jwt.Claims, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func parseAccessToken(tokenString, secret string) (*AccessClaims, error) {
	claims := &AccessClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

func parseRefreshToken(tokenString, secret string) (*RefreshClaims, error) {
	claims := &RefreshClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
