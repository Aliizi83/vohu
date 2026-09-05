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

func mapError(err error) (int, shared.ResultCode) {
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, shared.ResultNotFoundError
	case errors.Is(err, ErrUsernameTaken):
		return http.StatusConflict, shared.ResultConflictError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

func (h *Handler) Create(c *gin.Context) {
	shared.CreateHandler(c,
		shared.Identity[CreateUserRequest],
		shared.Identity[Response],
		h.service.Register,
		mapError,
	)
}

func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(u *User) Response { return toResponse(*u) },
		h.service.GetByID,
		mapError,
	)
}

func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateUserRequest],
		shared.Identity[Response],
		h.service.Update,
		mapError,
	)
}

func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.Delete, mapError)
}

func (h *Handler) List(c *gin.Context) {
	shared.ListHandler(c,
		func(u User) Response { return toResponse(u) },
		h.service.List,
	)
}
