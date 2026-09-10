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
//
//	@Summary		Get an agent tool
//	@Description	Returns one catalog entry by ID. Requires at least "read" access to this specific tool.
//	@Tags			agent-tools
//	@Produce		json
//	@Param			id	path		int	true	"Tool ID"
//	@Success		200	{object}	shared.BaseResponse{result=Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/agent-tools/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(t *Tool) Response { return toResponse(*t) },
		h.service.GetByID,
		mapError,
	)
}

// Update changes a tool's visibility.
//
//	@Summary		Update an agent tool's visibility
//	@Description	Changes a tool between public (available to everyone) and private (only whoever's been granted access). This is the only field the catalog lets an admin change — name/description/implemented are facts about the underlying Go implementation, not data this API owns. Requires "manage" access to this specific tool.
//	@Tags			agent-tools
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Tool ID"
//	@Param			request	body		UpdateToolRequest	true	"New visibility"
//	@Success		200		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/agent-tools/{id} [put]
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
//
//	@Summary		List agent tools
//	@Description	Lists every public tool plus any private tool the caller holds at least "read" access to. Every tool in this catalog is SSH-connection-bound — its Go implementation always takes a connectionId argument and checks the caller's access to that specific connection separately, on every call.
//	@Tags			agent-tools
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[Response]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/agent-tools [get]
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
