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
// Go forbids the reverse import. Wired into routes.go as middleware (see
// shared.RequireCustomCheckOnParam), not read by this handler directly.
type ConversationAccessCheck func(ctx context.Context, userID uint, conversationID uint) error

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

// Get returns one conversation's chat settings. Access is already decided
// by shared.RequireCustomCheckOnParam before this runs (see routes.go).
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
	shared.GetByIDHandler(c,
		func(s *ChatSetting) Response { return ToResponse(*s) },
		h.service.Get,
		mapError,
	)
}

// Update replaces one conversation's chat settings. Access is already
// decided by shared.RequireCustomCheckOnParam before this runs (see
// routes.go).
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
	shared.UpdateHandler(c,
		shared.Identity[UpdateChatSettingRequest],
		func(s *ChatSetting) Response { return ToResponse(*s) },
		h.service.Upsert,
		mapError,
	)
}
