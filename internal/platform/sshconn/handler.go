package sshconn

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
	case errors.Is(err, ErrConnectionTestFailed):
		return http.StatusBadRequest, shared.ResultValidationError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

// Create is hand-written rather than shared.CreateHandler because it needs
// the authenticated user's ID (to record who owns the new connection and
// auto-grant them access) — CreateHandler's create func has no room for
// that.
func (h *Handler) Create(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req CreateSSHConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	conn, err := h.service.Create(c.Request.Context(), userID, req)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toResponse(*conn))
}

// Get is the plain generic handler again — whether the caller may reach
// this specific connection at all is already decided by
// shared.RequireAccessLevelOnParam before this ever runs, so there's
// nothing left for the handler itself to check.
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(conn *SSHConnection) Response { return toResponse(*conn) },
		h.service.GetByID,
		mapError,
	)
}

func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateSSHConnectionRequest],
		func(conn *SSHConnection) Response { return toResponse(*conn) },
		h.service.Update,
		mapError,
	)
}

func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.Delete, mapError)
}

// List is hand-written for the same reason Get is — the caller's ID
// decides whether they see every connection or only the ones they hold
// resource-level access to (see Service.ListForCaller).
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
