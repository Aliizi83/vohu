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
		func(u *User) Response { return toResponse(*u) },
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
		func(u *User) Response { return toResponse(*u) },
		h.service.Update,
		mapError,
	)
}

func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.Delete, mapError)
}

// List is hand-written rather than shared.ListHandler because visibility
// isn't all-or-nothing anymore — Service.ListForCaller needs the caller's
// ID to decide whether they see every user or only the ones they hold
// resource-level access to.
func (h *Handler) List(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	page, filter, err := shared.ParseListQuery(c)
	if err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	items, total, err := h.service.ListForCaller(c.Request.Context(), userID, filter, page)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, toResponse(item))
	}

	shared.RespondSuccess(c, http.StatusOK, shared.NewPagedList(responses, total, page))
}
