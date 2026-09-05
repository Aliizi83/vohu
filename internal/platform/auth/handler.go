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
