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
var KnownResourceTypes = []string{"user", "role", "ssh_connection", "provider_key", "conversation", "resource_access"}

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

func (h *Handler) CreateRole(c *gin.Context) {
	shared.CreateHandler(c,
		shared.Identity[CreateRoleRequest],
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.CreateRole,
		mapError,
	)
}

func (h *Handler) GetRole(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.GetRole,
		mapError,
	)
}

func (h *Handler) UpdateRole(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateRoleRequest],
		func(r *Role) RoleResponse { return toRoleResponse(*r) },
		h.service.UpdateRole,
		mapError,
	)
}

func (h *Handler) DeleteRole(c *gin.Context) {
	shared.DeleteHandler(c, h.service.DeleteRole, mapError)
}

func (h *Handler) ListRoles(c *gin.Context) {
	shared.ListHandler(c,
		func(r Role) RoleResponse { return toRoleResponse(r) },
		h.service.ListRoles,
	)
}

// AssignRoleToUser handles POST /users/:id/roles — :id is the user ID.
// More than plain CRUD (a join-table write), so hand-written.
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
