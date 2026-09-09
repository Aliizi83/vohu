package custommodel

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

func (h *Handler) CreateMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req CreateCustomModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	resp, err := h.service.CreateMine(c.Request.Context(), userID, req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusCreated, resp)
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

	id, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	if err := h.service.DeleteMine(c.Request.Context(), userID, id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}

func (h *Handler) ListAccessible(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	items, err := h.service.ListAccessible(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

func (h *Handler) CreateGlobal(c *gin.Context) {
	var req CreateCustomModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	resp, err := h.service.CreateGlobal(c.Request.Context(), req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusCreated, resp)
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
	id, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	if err := h.service.DeleteGlobal(c.Request.Context(), id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}
