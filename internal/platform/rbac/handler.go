package rbac

import (
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// KnownResourceTypes is every resource type the platform currently
// defines — duplicated here as plain strings the same way
// seeders.KnownPermissions used to duplicate other modules' permission
// keys, so rbac never has to import sshconn/user/chat just to know their
// names. Used to seed the admin role's wildcard grants and to compute
// MyAccessResponse.Levels for /me/access.
var KnownResourceTypes = []string{"user", "role", "ssh_connection", "provider_key", "conversation", "resource_access", "custom_model", "agent_tool"}

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
	case errors.Is(err, ErrRoleExists):
		return http.StatusConflict, shared.ResultConflictError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

// CreateRole creates a new role.
//
//	@Summary		Create a role
//	@Description	Creates a new role. Requires wildcard "write" access on resource type "role".
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateRoleRequest	true	"New role"
//	@Success		201		{object}	shared.BaseResponse{result=RoleResponse}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		409		{object}	shared.BaseResponse	"A role with this name already exists"
//	@Security		BearerAuth
//	@Router			/roles [post]
func (h *Handler) CreateRole(c *gin.Context) {
	shared.CreateHandler(c,
		shared.Identity[CreateRoleRequest],
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.CreateRole,
		mapError,
	)
}

// GetRole returns one role by ID.
//
//	@Summary		Get a role
//	@Description	Returns one role by ID. Requires at least "read" access to this specific role.
//	@Tags			roles
//	@Produce		json
//	@Param			id	path		int	true	"Role ID"
//	@Success		200	{object}	shared.BaseResponse{result=RoleResponse}
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/roles/{id} [get]
func (h *Handler) GetRole(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.GetRole,
		mapError,
	)
}

// UpdateRole renames a role.
//
//	@Summary		Update a role
//	@Description	Renames a role. Requires "write" access to this specific role.
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"Role ID"
//	@Param			request	body		UpdateRoleRequest	true	"New name"
//	@Success		200		{object}	shared.BaseResponse{result=RoleResponse}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/roles/{id} [put]
func (h *Handler) UpdateRole(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateRoleRequest],
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.UpdateRole,
		mapError,
	)
}

// DeleteRole removes a role.
//
//	@Summary		Delete a role
//	@Description	Deletes a role. Requires "manage" access to this specific role. Users holding the role are not deleted, just lose whatever the role granted them.
//	@Tags			roles
//	@Produce		json
//	@Param			id	path		int	true	"Role ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/roles/{id} [delete]
func (h *Handler) DeleteRole(c *gin.Context) {
	shared.DeleteHandler(c, h.service.DeleteRole, mapError)
}

// ListRoles lists every role.
//
//	@Summary		List roles
//	@Description	Lists every role. Requires wildcard "read" access on resource type "role".
//	@Tags			roles
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[RoleResponse]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/roles [get]
func (h *Handler) ListRoles(c *gin.Context) {
	shared.ListHandler(c,
		func(r Role) RoleResponse { return toRoleResponse(r) },
		h.service.ListRoles,
	)
}

// AssignRoleToUser handles POST /users/:id/roles — :id is the user ID.
// More than plain CRUD (a join-table write), so hand-written.
//
//	@Summary		Assign a role to a user
//	@Description	Grants a user every resource access attached to the given role. Requires "manage" access to the target user.
//	@Tags			roles
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"User ID"
//	@Param			request	body		AssignRoleRequest	true	"Role to assign"
//	@Success		200		{object}	shared.BaseResponse
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/users/{id}/roles [post]
func (h *Handler) AssignRoleToUser(c *gin.Context) {
	userID, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.AssignRoleToUser(c.Request.Context(), userID, req.RoleID); err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

// GrantResourceAccess handles POST /resource-access. Hand-written rather
// than shared.CreateHandler for two reasons: the service method upserts
// (returns only an error, not the row), and — the important one — the
// resource being granted access to lives in the request body, not a route
// param, so the "does the granter already hold manage on this specific
// resource" check (granting access to X requires manage on X, no separate
// meta-permission needed) can't be expressed as route-level middleware
// the way every other check in this module is; it has to read the body
// first.
//
//	@Summary		Grant resource access
//	@Description	Grants (or upserts, if one already exists for the same grantee+resource) a resource-access row. The caller must already hold "manage" on the target resource — granting access to X requires manage on X, no separate meta-permission.
//	@Tags			resource-access
//	@Accept			json
//	@Produce		json
//	@Param			request	body		GrantResourceAccessRequest	true	"Grant"
//	@Success		200		{object}	shared.BaseResponse
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		403		{object}	shared.BaseResponse	"Caller doesn't hold manage on the target resource"
//	@Security		BearerAuth
//	@Router			/resource-access [post]
func (h *Handler) GrantResourceAccess(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req GrantResourceAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	allowed, err := h.service.HasAccessLevel(c.Request.Context(), userID, req.ResourceType, req.ResourceID, AccessManage)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	err = h.service.GrantResourceAccess(
		c.Request.Context(), req.GranteeType, req.GranteeID, req.ResourceType, req.ResourceID, req.Level, req.Effect,
	)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

// ListResourceAccess lists every resource-access grant.
//
//	@Summary		List resource-access grants
//	@Description	Lists every resource-access grant in the system. Requires wildcard "read" access on resource type "resource_access".
//	@Tags			resource-access
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[ResourceAccessResponse]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/resource-access [get]
func (h *Handler) ListResourceAccess(c *gin.Context) {
	shared.ListHandler(c,
		func(a ResourceAccess) ResourceAccessResponse { return toResourceAccessResponse(a) },
		h.service.ListResourceAccess,
	)
}

// RevokeResourceAccess handles DELETE /resource-access/:id — :id is the
// grant row's own ID, not the resource it refers to, so the "does the
// caller hold manage on that resource" check has to load the row first to
// learn its ResourceType/ResourceID, same reasoning as GrantResourceAccess
// above.
//
//	@Summary		Revoke a resource-access grant
//	@Description	Deletes one resource-access grant by its own ID. The caller must hold "manage" on the resource that grant refers to.
//	@Tags			resource-access
//	@Produce		json
//	@Param			id	path		int	true	"Resource-access grant ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		403	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/resource-access/{id} [delete]
func (h *Handler) RevokeResourceAccess(c *gin.Context) {
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

	grant, err := h.service.GetResourceAccess(c.Request.Context(), id)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	allowed, err := h.service.HasAccessLevel(c.Request.Context(), userID, grant.ResourceType, grant.ResourceID, AccessManage)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	if !allowed {
		shared.RespondError(c, http.StatusForbidden, shared.ResultForbiddenError, errors.New("forbidden"))
		return
	}

	if err := h.service.RevokeResourceAccess(c.Request.Context(), id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

// GetMyAccess handles GET /me/access — self-service, no policy beyond
// being authenticated, since it only ever returns the caller's own data.
//
//	@Summary		Get my access profile
//	@Description	Returns the caller's own complete access profile — every resource-access grant that's personally theirs (direct or via a role they hold), plus the best level they hold on each known resource type at large. The frontend uses this right after login to decide what nav items and buttons to show; it's a UX layer, every endpoint still enforces its own access independently regardless of what this reports.
//	@Tags			resource-access
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=MyAccessResponse}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/me/access [get]
func (h *Handler) GetMyAccess(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	grants, err := h.service.MyResourceAccess(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}
	resourceAccess := make([]ResourceAccessResponse, 0, len(grants))
	for _, g := range grants {
		resourceAccess = append(resourceAccess, toResourceAccessResponse(g))
	}

	levels := make(map[string]AccessLevel, len(KnownResourceTypes))
	for _, resourceType := range KnownResourceTypes {
		level, ok, err := h.service.MyLevel(c.Request.Context(), userID, resourceType)
		if err != nil {
			shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
			return
		}
		if ok {
			levels[resourceType] = level
		}
	}

	shared.RespondSuccess(c, http.StatusOK, MyAccessResponse{
		ResourceAccess: resourceAccess,
		Levels:         levels,
	})
}
