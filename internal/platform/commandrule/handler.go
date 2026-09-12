package commandrule

import (
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// sshConnectionResourceType is sshconn.ResourceTypeSSHConnection's value,
// duplicated as a literal rather than imported — this package, like every
// platform module besides chat, never imports another platform module
// directly; a resource type crosses module boundaries as a plain string,
// the same way rbac.AccessLevel does via shared.AccessLevelCheck.
const sshConnectionResourceType = "ssh_connection"

type Handler struct {
	service        Service
	hasAccessLevel shared.AccessLevelCheck
}

func NewHandler(service Service, hasAccessLevel shared.AccessLevelCheck) *Handler {
	return &Handler{service: service, hasAccessLevel: hasAccessLevel}
}

func mapError(err error) (int, shared.ResultCode) {
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, shared.ResultNotFoundError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

// Create is hand-written rather than shared.CreateHandler for the same
// reason rbac.Handler.GrantResourceAccess is: the connection a new rule
// belongs to lives in the request body, not a route param, so "does the
// caller already hold manage on that connection" can't be expressed as
// route-level middleware — it has to read the body first.
//
//	@Summary		Create a command rule
//	@Description	Adds one allow/deny rule to an SSH connection's command policy. Rules are first-match-wins within a connection (evaluated in creation order) — a connection with no rules permits nothing. The caller must already hold "manage" on the target connection, same level required to delete it.
//	@Tags			command-rules
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateRuleRequest	true	"New rule"
//	@Success		201		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		403		{object}	shared.BaseResponse	"Caller doesn't hold manage on the target connection"
//	@Security		BearerAuth
//	@Router			/command-rules [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	allowed, err := h.hasAccessLevel(c.Request.Context(), userID, sshConnectionResourceType, req.SSHConnectionID, "manage")
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	rule, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toResponse(*rule))
}

type listQuery struct {
	SSHConnectionID uint `form:"sshConnectionId" binding:"required"`
	PageNumber      int  `form:"pageNumber"`
	PageSize        int  `form:"pageSize"`
}

// List is hand-written for the same reason Create is — the connection
// being asked about is a query param, not a route param.
//
//	@Summary		List an SSH connection's command rules
//	@Description	Lists every command rule for one SSH connection. The caller must hold "manage" on that connection.
//	@Tags			command-rules
//	@Produce		json
//	@Param			sshConnectionId	query		int	true	"SSH connection ID"
//	@Param			pageNumber		query		int	false	"Page number, default 1"
//	@Param			pageSize		query		int	false	"Page size, default 10"
//	@Success		200				{object}	shared.BaseResponse{result=shared.PagedList[Response]}
//	@Failure		400				{object}	shared.BaseResponse
//	@Failure		401				{object}	shared.BaseResponse
//	@Failure		403				{object}	shared.BaseResponse	"Caller doesn't hold manage on the target connection"
//	@Security		BearerAuth
//	@Router			/command-rules [get]
func (h *Handler) List(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var q listQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	allowed, err := h.hasAccessLevel(c.Request.Context(), userID, sshConnectionResourceType, q.SSHConnectionID, "manage")
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	page := shared.Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}
	items, total, err := h.service.ListForConnection(c.Request.Context(), q.SSHConnectionID, page)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	responses := make([]Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, toResponse(item))
	}

	shared.RespondSuccess(c, http.StatusOK, shared.NewPagedList(responses, total, page))
}

// Update handles PUT /command-rules/:id — :id is the rule's own ID, so
// (same reasoning as rbac.Handler.RevokeResourceAccess) the rule has to
// be loaded first to learn which connection it belongs to before the
// access check can run.
//
//	@Summary		Update a command rule
//	@Description	Changes a rule's program/argsPrefixes/allowed. The caller must hold "manage" on the connection this rule belongs to.
//	@Tags			command-rules
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Rule ID"
//	@Param			request	body		UpdateRuleRequest	true	"Fields to update"
//	@Success		200		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		403		{object}	shared.BaseResponse	"Caller doesn't hold manage on the connection this rule belongs to"
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/command-rules/{id} [put]
func (h *Handler) Update(c *gin.Context) {
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

	rule, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	allowed, err := h.hasAccessLevel(c.Request.Context(), userID, sshConnectionResourceType, rule.SSHConnectionID, "manage")
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	var req UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	updated, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, toResponse(*updated))
}

// Delete handles DELETE /command-rules/:id — same reasoning as Update for
// loading the rule before checking access.
//
//	@Summary		Delete a command rule
//	@Description	Removes one rule from an SSH connection's command policy. The caller must hold "manage" on the connection this rule belongs to.
//	@Tags			command-rules
//	@Produce		json
//	@Param			id	path		int	true	"Rule ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		403	{object}	shared.BaseResponse	"Caller doesn't hold manage on the connection this rule belongs to"
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/command-rules/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
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

	rule, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	allowed, err := h.hasAccessLevel(c.Request.Context(), userID, sshConnectionResourceType, rule.SSHConnectionID, "manage")
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}
