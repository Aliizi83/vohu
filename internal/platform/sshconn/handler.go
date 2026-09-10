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
//
//	@Summary		Create an SSH connection
//	@Description	Registers a new SSH connection. Private-key auth only — the connection is test-dialed (real handshake, no command run) before it's saved, so a bad host or mismatched key fails loudly here instead of the first time the agent tries to use it. The key is encrypted at rest and never returned by any response. The creator is auto-granted "manage" on the new connection. Requires wildcard "write" access on resource type "ssh_connection".
//	@Tags			ssh-connections
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateSSHConnectionRequest	true	"New connection"
//	@Success		201		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse	"Validation error, or the connection test failed (bad host/key)"
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections [post]
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
//
//	@Summary		Get an SSH connection
//	@Description	Returns one SSH connection by ID (never the private key). Requires at least "read" access to this specific connection.
//	@Tags			ssh-connections
//	@Produce		json
//	@Param			id	path		int	true	"Connection ID"
//	@Success		200	{object}	shared.BaseResponse{result=Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(conn *SSHConnection) Response { return toResponse(*conn) },
		h.service.GetByID,
		mapError,
	)
}

// Update changes an SSH connection's fields.
//
//	@Summary		Update an SSH connection
//	@Description	Changes an SSH connection's fields. Omitting privateKey keeps the existing key (there's no way to show it back to the client to prefill a form). Requires "write" access to this specific connection.
//	@Tags			ssh-connections
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Connection ID"
//	@Param			request	body		UpdateSSHConnectionRequest	true	"Fields to update"
//	@Success		200		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateSSHConnectionRequest],
		func(conn *SSHConnection) Response { return toResponse(*conn) },
		h.service.Update,
		mapError,
	)
}

// Delete removes an SSH connection.
//
//	@Summary		Delete an SSH connection
//	@Description	Deletes an SSH connection. Requires "manage" access to this specific connection.
//	@Tags			ssh-connections
//	@Produce		json
//	@Param			id	path		int	true	"Connection ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.Delete, mapError)
}

// List is hand-written for the same reason Get is — the caller's ID
// decides whether they see every connection or only the ones they hold
// resource-level access to (see Service.ListForCaller).
//
//	@Summary		List SSH connections
//	@Description	Lists SSH connections the caller holds at least "read" access to — every connection, if the caller holds a wildcard "read"/"manage" grant on resource type "ssh_connection".
//	@Tags			ssh-connections
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[Response]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/ssh-connections [get]
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
