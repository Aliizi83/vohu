package auth

import "time"

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

// TokenPair is the service-level result of a successful login/refresh.
type TokenPair struct {
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type TokenResponse struct {
	AccessToken           string `json:"accessToken"`
	AccessTokenExpiresAt  int64  `json:"accessTokenExpiresAt"`
	RefreshToken          string `json:"refreshToken"`
	RefreshTokenExpiresAt int64  `json:"refreshTokenExpiresAt"`
}

func toTokenResponse(t TokenPair) TokenResponse {
	return TokenResponse{
		AccessToken:           t.AccessToken,
		AccessTokenExpiresAt:  t.AccessTokenExpiresAt.Unix(),
		RefreshToken:          t.RefreshToken,
		RefreshTokenExpiresAt: t.RefreshTokenExpiresAt.Unix(),
	}
}
