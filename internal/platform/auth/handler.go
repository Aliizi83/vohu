package auth

import (
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// Login authenticates a username/password and issues a fresh token pair.
//
//	@Summary		Log in
//	@Description	Exchanges a username and password for an access/refresh token pair. No auth required — this is the entry point that produces tokens.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Credentials"
//	@Success		200		{object}	shared.BaseResponse{result=TokenResponse}
//	@Failure		400		{object}	shared.BaseResponse	"Missing username or password"
//	@Failure		401		{object}	shared.BaseResponse	"Invalid username or password"
//	@Router			/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	tokens, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			shared.RespondError(c, http.StatusUnauthorized, shared.ResultAuthError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, toTokenResponse(tokens))
}

// Refresh exchanges a still-valid refresh token for a brand new
// access/refresh pair.
//
//	@Summary		Refresh a token pair
//	@Description	Exchanges a valid refresh token for a new access/refresh token pair. No auth header required — the refresh token in the body is the credential.
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RefreshRequest	true	"Refresh token"
//	@Success		200		{object}	shared.BaseResponse{result=TokenResponse}
//	@Failure		400		{object}	shared.BaseResponse	"Missing refresh token"
//	@Failure		401		{object}	shared.BaseResponse	"Invalid or expired refresh token"
//	@Router			/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	tokens, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		shared.RespondError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("invalid refresh token"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, toTokenResponse(tokens))
}
