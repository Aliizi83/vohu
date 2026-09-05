package user

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

func (h *Handler) Create(c *gin.Context) {
	var req CreateUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	res, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrUsernameTaken) {
			shared.RespondError(c, http.StatusConflict, shared.ResultConflictError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, res)
}

func (h *Handler) Get(c *gin.Context) {
	id, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	u, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, toResponse(*u))
}
