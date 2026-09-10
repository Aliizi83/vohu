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

// Create registers a new user account.
//
//	@Summary		Create a user
//	@Description	Creates a new user account. Requires wildcard "write" access on resource type "user".
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateUserRequest	true	"New user"
//	@Success		201		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse	"Validation error"
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		409		{object}	shared.BaseResponse	"Username already taken"
//	@Security		BearerAuth
//	@Router			/users [post]
func (h *Handler) Create(c *gin.Context) {
	shared.CreateHandler(c,
		shared.Identity[CreateUserRequest],
		func(u *User) Response { return toResponse(*u) },
		h.service.Register,
		mapError,
	)
}

// Get returns one user by ID.
//
//	@Summary		Get a user
//	@Description	Returns one user by ID. Requires at least "read" access to this specific user (a direct grant, a wildcard grant, or a grant on a role this user holds).
//	@Tags			users
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	shared.BaseResponse{result=Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse	"Not found, or not permitted (existence isn't leaked to a caller with no access)"
//	@Security		BearerAuth
//	@Router			/users/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(u *User) Response { return toResponse(*u) },
		h.service.GetByID,
		mapError,
	)
}

// Update changes a user's email and/or enabled flag.
//
//	@Summary		Update a user
//	@Description	Changes a user's email and/or enabled flag. Requires "write" access to this specific user.
//	@Tags			users
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int					true	"User ID"
//	@Param			request	body		UpdateUserRequest	true	"Fields to update"
//	@Success		200		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/users/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateUserRequest],
		func(u *User) Response { return toResponse(*u) },
		h.service.Update,
		mapError,
	)
}

// Delete removes a user account.
//
//	@Summary		Delete a user
//	@Description	Deletes a user account. Requires "manage" access to this specific user.
//	@Tags			users
//	@Produce		json
//	@Param			id	path		int	true	"User ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/users/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.Delete, mapError)
}

// List is hand-written rather than shared.ListHandler because visibility
// isn't all-or-nothing anymore — Service.ListForCaller needs the caller's
// ID to decide whether they see every user or only the ones they hold
// resource-level access to.
//
//	@Summary		List users
//	@Description	Lists users the caller holds at least "read" access to — every user, if the caller holds a wildcard "read"/"manage" grant on resource type "user". filter is a JSON-encoded shared.DynamicFilter.
//	@Tags			users
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[Response]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/users [get]
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
