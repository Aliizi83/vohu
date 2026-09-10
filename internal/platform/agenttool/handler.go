package agenttool

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

// Get/Update are the plain generic handlers — access is already decided
// by shared.RequireAccessLevelOnParam before these run, see routes.go.
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(t *Tool) Response { return toResponse(*t) },
		h.service.GetByID,
		mapError,
	)
}

func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateToolRequest],
		func(t *Tool) Response { return toResponse(*t) },
		h.service.Update,
		mapError,
	)
}

// List is hand-written for the same reason sshconn.Handler.List is — the
// caller's ID decides whether they see every tool or only the ones
// they're allowed to use (see Service.ListForCaller).
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
