package custommodel

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

// CreateMine adds one of the caller's own named OpenAI-compatible presets.
//
//	@Summary		Create my custom model preset
//	@Description	Adds a named OpenAI-compatible preset (URL + key + model) owned by the caller — unlike provider-keys' single "openai" slot, any number of these can coexist (a local Ollama server and a DeepSeek account at once, say). The key is encrypted at rest and never returned by any response.
//	@Tags			custom-models
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateCustomModelRequest	true	"New preset"
//	@Success		201		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models/me [post]
func (h *Handler) CreateMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req CreateCustomModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	resp, err := h.service.CreateMine(c.Request.Context(), userID, req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusCreated, resp)
}

// ListMine lists the caller's own custom model presets.
//
//	@Summary		List my custom model presets
//	@Description	Lists the caller's own named OpenAI-compatible presets (never the API key values).
//	@Tags			custom-models
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=[]Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models/me [get]
func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	items, err := h.service.ListMine(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

// DeleteMine removes one of the caller's own custom model presets.
//
//	@Summary		Delete my custom model preset
//	@Description	Deletes one of the caller's own custom model presets by ID.
//	@Tags			custom-models
//	@Produce		json
//	@Param			id	path		int	true	"Preset ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models/me/{id} [delete]
func (h *Handler) DeleteMine(c *gin.Context) {
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

	if err := h.service.DeleteMine(c.Request.Context(), userID, id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}

// ListAccessible is the new-chat picker's read path.
//
//	@Summary		List custom models available to me
//	@Description	Lists every custom model preset the caller can pick when starting a new chat — their own presets plus every global one — regardless of whether they're allowed to manage global presets. Deliberately not gated by "manage" access, unlike the other routes in this module.
//	@Tags			custom-models
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=[]Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models/available [get]
func (h *Handler) ListAccessible(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	items, err := h.service.ListAccessible(c.Request.Context(), userID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

// CreateGlobal adds a custom model preset every user can pick.
//
//	@Summary		Create a global custom model preset
//	@Description	Adds a named OpenAI-compatible preset every user can pick when starting a new chat, not just the creator. The key is encrypted at rest and never returned by any response. Requires wildcard "manage" access on resource type "custom_model".
//	@Tags			custom-models
//	@Accept			json
//	@Produce		json
//	@Param			request	body		CreateCustomModelRequest	true	"New preset"
//	@Success		201		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models [post]
func (h *Handler) CreateGlobal(c *gin.Context) {
	var req CreateCustomModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	resp, err := h.service.CreateGlobal(c.Request.Context(), req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusCreated, resp)
}

// ListGlobal lists every global custom model preset.
//
//	@Summary		List global custom model presets
//	@Description	Lists every account-wide custom model preset (never the API key values). Requires wildcard "manage" access on resource type "custom_model".
//	@Tags			custom-models
//	@Produce		json
//	@Success		200	{object}	shared.BaseResponse{result=[]Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models [get]
func (h *Handler) ListGlobal(c *gin.Context) {
	items, err := h.service.ListGlobal(c.Request.Context())
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, items)
}

// DeleteGlobal removes a global custom model preset.
//
//	@Summary		Delete a global custom model preset
//	@Description	Deletes an account-wide custom model preset by ID. Requires wildcard "manage" access on resource type "custom_model".
//	@Tags			custom-models
//	@Produce		json
//	@Param			id	path		int	true	"Preset ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/custom-models/{id} [delete]
func (h *Handler) DeleteGlobal(c *gin.Context) {
	id, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	if err := h.service.DeleteGlobal(c.Request.Context(), id); err != nil {
		status, code := mapError(err)
		shared.RespondError(c, status, code, err)
		return
	}
	shared.RespondSuccess(c, http.StatusOK, nil)
}
