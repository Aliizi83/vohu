package providerkey

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

func mapError(err error) (int, shared.ResultCode) {
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, shared.ResultNotFoundError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

func (h *Handler) SetGlobal(c *gin.Context) {
	var req SetKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.SetGlobal(c.Request.Context(), req); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

func (h *Handler) ListGlobal(c *gin.Context) {
	items, err := h.service.ListGlobal(c.Request.Context())
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

func (h *Handler) DeleteGlobal(c *gin.Context) {
	provider := Provider(c.Param("provider"))
	if err := h.service.DeleteGlobal(c.Request.Context(), provider); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}

func (h *Handler) SetMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req SetKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.SetMine(c.Request.Context(), userID, req); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	items, err := h.service.ListMine(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

func (h *Handler) DeleteMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	provider := Provider(c.Param("provider"))
	if err := h.service.DeleteMine(c.Request.Context(), userID, provider); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}
