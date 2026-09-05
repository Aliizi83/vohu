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

func (h *Handler) CreateRole(c *gin.Context) {
	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	role, err := h.service.EnsureRole(c.Request.Context(), req.Name)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toRoleResponse(*role))
}

func (h *Handler) CreatePermission(c *gin.Context) {
	var req CreatePermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	permission, err := h.service.EnsurePermission(c.Request.Context(), req.Key)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toPermissionResponse(*permission))
}

func (h *Handler) ListPermissions(c *gin.Context) {
	permissions, err := h.service.ListAllPermissions(c.Request.Context())
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, errors.New("internal error"))
		return
	}

	responses := make([]PermissionResponse, 0, len(permissions))
	for _, p := range permissions {
		responses = append(responses, toPermissionResponse(p))
	}

	shared.RespondSuccess(c, http.StatusOK, responses)
}

// GrantPermissionToRole handles POST /roles/:id/permissions — :id is the role ID.
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
