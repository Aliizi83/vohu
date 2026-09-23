package customtool

import (
	"errors"
	"net/http"
	"regexp"
	"runtime"
	"strconv"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/toolbuild"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
	builder toolbuild.Builder
}

func NewHandler(service Service, builder toolbuild.Builder) *Handler {
	return &Handler{service: service, builder: builder}
}

func mapError(err error) (int, shared.ResultCode) {
	switch {
	case errors.Is(err, shared.ErrNotFound):
		return http.StatusNotFound, shared.ResultNotFoundError
	default:
		return http.StatusInternalServerError, shared.ResultInternalError
	}
}

// @Summary		Create a custom tool
// @Description	Registers a new user/agent-authored tool definition. The creator is auto-granted "manage" on the new tool. Requires wildcard "write" access on resource type "custom_tool".
// @Tags			custom-tools
// @Accept			json
// @Produce		json
// @Param			request	body		CreateToolRequest	true	"New tool"
// @Success		201		{object}	shared.BaseResponse{result=Response}
// @Failure		400		{object}	shared.BaseResponse
// @Failure		401		{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools [post]
func (h *Handler) Create(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	var req CreateToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	tool, err := h.service.CreateTool(c.Request.Context(), userID, req)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toResponse(*tool))
}

// @Summary		Get a custom tool
// @Description	Returns one tool's metadata by ID (not its source — see GET /custom-tools/{id}/versions). Requires at least "read" access to this specific tool.
// @Tags			custom-tools
// @Produce		json
// @Param			id	path		int	true	"Tool ID"
// @Success		200	{object}	shared.BaseResponse{result=Response}
// @Failure		401	{object}	shared.BaseResponse
// @Failure		404	{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	shared.GetByIDHandler(c,
		func(t *Tool) Response { return toResponse(*t) },
		h.service.GetToolByID,
		mapError,
	)
}

// @Summary		Update a custom tool
// @Description	Changes a tool's description, params schema, or visibility. Name can't be changed here — see UpdateToolRequest's doc comment. Requires "write" access to this specific tool.
// @Tags			custom-tools
// @Accept			json
// @Produce		json
// @Param			id		path		int					true	"Tool ID"
// @Param			request	body		UpdateToolRequest	true	"Fields to update"
// @Success		200		{object}	shared.BaseResponse{result=Response}
// @Failure		400		{object}	shared.BaseResponse
// @Failure		401		{object}	shared.BaseResponse
// @Failure		404		{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	shared.UpdateHandler(c,
		shared.Identity[UpdateToolRequest],
		func(t *Tool) Response { return toResponse(*t) },
		h.service.UpdateTool,
		mapError,
	)
}

// @Summary		Delete a custom tool
// @Description	Deletes a tool and all of its versions. Requires "manage" access to this specific tool.
// @Tags			custom-tools
// @Produce		json
// @Param			id	path		int	true	"Tool ID"
// @Success		200	{object}	shared.BaseResponse
// @Failure		401	{object}	shared.BaseResponse
// @Failure		404	{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	shared.DeleteHandler(c, h.service.DeleteTool, mapError)
}

// @Summary		List custom tools
// @Description	Lists every public tool plus any private tool the caller holds at least "read" access to.
// @Tags			custom-tools
// @Produce		json
// @Param			pageNumber	query		int		false	"Page number, default 1"
// @Param			pageSize	query		int		false	"Page size, default 10"
// @Param			filter		query		string	false	"JSON-encoded shared.DynamicFilter"
// @Success		200			{object}	shared.BaseResponse{result=shared.PagedList[Response]}
// @Failure		401			{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools [get]
func (h *Handler) List(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	page, filter, err := shared.ParseListQuery(c)
	if err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	items, total, err := h.service.ListToolsForCaller(c.Request.Context(), userID, filter, page)
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

// goErrorPattern matches the standard `go build` diagnostic line format —
// e.g. "./main.go:5:2: undefined: fmt" — the same shape toolbuild.GoBuilder
// produces since it always compiles a file named main.go from its own temp
// dir.
var goErrorPattern = regexp.MustCompile(`(?m)^\./main\.go:(\d+):(\d+): (.+)$`)

// @Summary		Check a Go source string for build errors
// @Description	Compiles the given source exactly as it would be compiled for a real deploy (see internal/toolbuild), for the host's own OS/arch — not tied to any existing tool or version, so it can be called before either exists yet. Never fails with an HTTP error for a source that doesn't compile; that's reported as success=false with line/column diagnostics instead. Requires wildcard "write" access on resource type "custom_tool", same bar as creating a tool.
// @Tags			custom-tools
// @Accept			json
// @Produce		json
// @Param			request	body		CheckSourceRequest	true	"Source to check"
// @Success		200		{object}	shared.BaseResponse{result=CheckSourceResponse}
// @Failure		400		{object}	shared.BaseResponse
// @Failure		401		{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/check-source [post]
func (h *Handler) CheckSource(c *gin.Context) {
	var req CheckSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	_, err := h.builder.Build(c.Request.Context(), toolbuild.Request{
		SourceCode: req.SourceCode,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
	})
	if err == nil {
		shared.RespondSuccess(c, http.StatusOK, CheckSourceResponse{Success: true})
		return
	}

	shared.RespondSuccess(c, http.StatusOK, CheckSourceResponse{Success: false, Diagnostics: ParseGoErrors(err.Error())})
}

func ParseGoErrors(raw string) []SourceDiagnostic {
	matches := goErrorPattern.FindAllStringSubmatch(raw, -1)
	if matches == nil {
		return []SourceDiagnostic{{Line: 1, Column: 1, Message: raw}}
	}

	diagnostics := make([]SourceDiagnostic, 0, len(matches))
	for _, m := range matches {
		line, _ := strconv.Atoi(m[1])
		column, _ := strconv.Atoi(m[2])
		diagnostics = append(diagnostics, SourceDiagnostic{Line: line, Column: column, Message: m[3]})
	}
	return diagnostics
}

// @Summary		Add a new version to a custom tool
// @Description	Adds a new immutable version (source code) to an existing tool. Versions are never edited once created — a change is always a new version. Requires "manage" access to the tool.
// @Tags			custom-tools
// @Accept			json
// @Produce		json
// @Param			id		path		int						true	"Tool ID"
// @Param			request	body		CreateVersionRequest	true	"New version"
// @Success		201		{object}	shared.BaseResponse{result=VersionResponse}
// @Failure		400		{object}	shared.BaseResponse
// @Failure		401		{object}	shared.BaseResponse
// @Failure		404		{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/{id}/versions [post]
func (h *Handler) CreateVersion(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	toolID, ok := shared.RequireIDParam(c)
	if !ok {
		return
	}

	var req CreateVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	version, err := h.service.CreateVersion(c.Request.Context(), userID, toolID, req)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, toVersionResponse(*version))
}

// @Summary		List a custom tool's versions
// @Description	Lists every version of a tool, most recently created first. Requires "read" access to the tool.
// @Tags			custom-tools
// @Produce		json
// @Param			id			path		int	true	"Tool ID"
// @Param			pageNumber	query		int	false	"Page number, default 1"
// @Param			pageSize	query		int	false	"Page size, default 10"
// @Success		200			{object}	shared.BaseResponse{result=shared.PagedList[VersionResponse]}
// @Failure		401			{object}	shared.BaseResponse
// @Failure		404			{object}	shared.BaseResponse
// @Security		BearerAuth
// @Router			/custom-tools/{id}/versions [get]
func (h *Handler) ListVersions(c *gin.Context) {
	toolID, ok := shared.RequireIDParam(c)
	if !ok {
		return
	}

	var q struct {
		PageNumber int `form:"pageNumber"`
		PageSize   int `form:"pageSize"`
	}
	if err := c.ShouldBindQuery(&q); err != nil {
		shared.RespondValidationError(c, err)
		return
	}
	page := shared.Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}

	items, total, err := h.service.ListVersionsForTool(c.Request.Context(), toolID, page)
	if err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}

	responses := make([]VersionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toVersionResponse(item))
	}

	shared.RespondSuccess(c, http.StatusOK, shared.NewPagedList(responses, total, page))
}
