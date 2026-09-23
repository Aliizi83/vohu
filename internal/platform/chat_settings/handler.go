package chat_settings

import (
	"context"
	"errors"
	"net/http"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/gin-gonic/gin"
)

// ConversationAccessCheck confirms the caller may act on a conversation —
// injected from main.go's composition root rather than imported directly,
// since conversation.Service already imports chat_settings.Repository and
// Go forbids the reverse import.
type ConversationAccessCheck func(ctx context.Context, userID uint, conversationID uint) error

type Handler struct {
	service   Service
	canAccess ConversationAccessCheck
}

func NewHandler(service Service, canAccess ConversationAccessCheck) *Handler {
	return &Handler{service: service, canAccess: canAccess}
}

// Get returns one conversation's chat settings.
//
//	@Summary		Get a conversation's chat settings
//	@Description	Returns the calling conversation's own settings (max tool calls per turn, default prompt addition). The caller must own the conversation, or hold "read" access to it.
//	@Tags			chat
//	@Produce		json
//	@Param			id	path		int	true	"Conversation ID"
//	@Success		200	{object}	shared.BaseResponse{result=Response}
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id}/settings [get]
func (h *Handler) Get(c *gin.Context) {
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

	if err := h.canAccess(c.Request.Context(), userID, id); err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	settings, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, ToResponse(*settings))
}

// Update replaces one conversation's chat settings.
//
//	@Summary		Update a conversation's chat settings
//	@Description	Replaces the calling conversation's max tool calls per turn and default prompt addition. The caller must own the conversation, or hold "read" access to it.
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int							true	"Conversation ID"
//	@Param			request	body		UpdateChatSettingRequest	true	"Fields to update"
//	@Success		200		{object}	shared.BaseResponse{result=Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id}/settings [put]
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

	var req UpdateChatSettingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	if err := h.canAccess(c.Request.Context(), userID, id); err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	settings, err := h.service.Upsert(c.Request.Context(), id, req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, ToResponse(*settings))
}
