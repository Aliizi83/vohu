package rbac

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
	case errors.Is(err, ErrRoleExists), errors.Is(err, ErrPermissionExists):
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

func (h *Handler) CreatePermission(c *gin.Context) {
	shared.CreateHandler(c,
		shared.Identity[CreatePermissionRequest],
		func(p *Permission) PermissionResponse { return toPermissionResponse(*p) },
		h.service.CreatePermission,
		mapError,
	)
}

func (h *Handler) GetPermission(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(p *Permission) PermissionResponse { return toPermissionResponse(*p) },
		h.service.GetPermission,
		mapError,
	)
}

func (h *Handler) UpdatePermission(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdatePermissionRequest],
		func(p *Permission) PermissionResponse { return toPermissionResponse(*p) },
		h.service.UpdatePermission,
		mapError,
	)
}

func (h *Handler) DeletePermission(c *gin.Context) {
	shared.DeleteHandler(c, h.service.DeletePermission, mapError)
}

func (h *Handler) ListPermissions(c *gin.Context) {
	shared.ListHandler(c,
		func(p Permission) PermissionResponse { return toPermissionResponse(p) },
		h.service.ListPermissions,
	)
}

// GrantPermissionToRole handles POST /roles/:id/permissions — :id is the
// role ID. More than plain CRUD (a join-table write), so hand-written.
func (h *Handler) GrantPermissionToRole(c *gin.Context) {
	roleID, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	var req GrantPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.service.GrantPermissionToRole(c.Request.Context(), roleID, req.PermissionID); err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
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

// GrantResourceAccess handles POST /resource-permissions. Hand-written
// rather than shared.CreateHandler because the service method upserts
// (returns only an error, not the row) and the four fields all live
// directly on the request body rather than one coming from a route param.
func (h *Handler) GrantResourceAccess(c *gin.Context) {
	var req GrantResourceAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	err := h.service.GrantResourceAccess(c.Request.Context(), req.UserID, req.ResourceType, req.ResourceID, req.Level)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
}

func (h *Handler) ListResourcePermissions(c *gin.Context) {
	shared.ListHandler(c,
		func(p ResourcePermission) ResourcePermissionResponse { return toResourcePermissionResponse(p) },
		h.service.ListResourcePermissions,
	)
}

func (h *Handler) RevokeResourceAccess(c *gin.Context) {
	shared.DeleteHandler(c, h.service.RevokeResourceAccess, mapError)
}

// GetMyAccess handles GET /me/access — self-service, no policy beyond
// being authenticated, since it only ever returns the caller's own data.
func (h *Handler) GetMyAccess(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	permissions, err := h.service.ListPermissionKeysForUser(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	grants, err := h.service.ListResourceAccessForUser(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	resourceAccess := make([]ResourcePermissionResponse, 0, len(grants))
	for _, g := range grants {
		resourceAccess = append(resourceAccess, toResourcePermissionResponse(g))
	}

	shared.RespondSuccess(c, http.StatusOK, MyAccessResponse{
		Permissions:    permissions,
		ResourceAccess: resourceAccess,
	})
}
