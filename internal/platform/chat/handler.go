package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Aliizi83/vohu/internal/agent"
	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"github.com/Aliizi83/vohu/internal/platform/conversation"
	"github.com/Aliizi83/vohu/internal/platform/custommodel"
	"github.com/Aliizi83/vohu/internal/platform/providerkey"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"github.com/gin-gonic/gin"
)

// Handler is the only place in internal/platform that imports
// internal/agent, internal/tools, internal/tools/command, and
// internal/ai_model/models (via llm.go) — every other platform module
// stays free of any dependency on Vohu's actual agent core.
type Handler struct {
	conversations conversation.Service
	sshconns      sshconn.Service
	canAccess     shared.AccessLevelCheck
	commandPolicy command.Policy
	providerKeys  providerkey.Service
	customModels  custommodel.Service
	agentTools    agenttool.Service
}

func NewHandler(
	conversations conversation.Service,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandPolicy command.Policy,
	providerKeys providerkey.Service,
	customModels custommodel.Service,
	agentTools agenttool.Service,
) *Handler {
	return &Handler{
		conversations: conversations,
		sshconns:      sshconns,
		canAccess:     canAccess,
		commandPolicy: commandPolicy,
		providerKeys:  providerKeys,
		customModels:  customModels,
		agentTools:    agentTools,
	}
}

// CreateConversation starts a new conversation.
//
//	@Summary		Create a conversation
//	@Description	Starts a new conversation — provider and model are fixed for its whole lifetime once created. customModelId, if set, names one of the caller's (or a global) custom model presets to use instead of the account's single per-provider key.
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			request	body		conversation.CreateConversationRequest	true	"New conversation"
//	@Success		201		{object}	shared.BaseResponse{result=conversation.Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations [post]
func (h *Handler) CreateConversation(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var req conversation.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	conv, err := h.conversations.Create(c.Request.Context(), userID, req)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusCreated, conversation.ToResponse(*conv))
}

type listConversationsQuery struct {
	PageNumber int `form:"pageNumber"`
	PageSize   int `form:"pageSize"`
}

// ListConversations lists the caller's own conversations.
//
//	@Summary		List my conversations
//	@Description	Lists the caller's own conversations. There's no separate access-policy gate here beyond being authenticated — conversations are scoped to their owner by construction.
//	@Tags			chat
//	@Produce		json
//	@Param			pageNumber	query		int	false	"Page number, default 1"
//	@Param			pageSize	query		int	false	"Page size, default 10"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[conversation.Response]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations [get]
func (h *Handler) ListConversations(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	var q listConversationsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		shared.RespondValidationError(c, err)
		return
	}
	page := shared.Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}

	items, total, err := h.conversations.List(c.Request.Context(), userID, page)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	responses := make([]conversation.Response, 0, len(items))
	for _, item := range items {
		responses = append(responses, conversation.ToResponse(item))
	}

	shared.RespondSuccess(c, http.StatusOK, shared.NewPagedList(responses, total, page))
}

type listMessagesQuery struct {
	PageNumber int `form:"pageNumber"`
	PageSize   int `form:"pageSize"`
}

// GetMessages returns one page of a conversation's messages — page 1 is
// the most recent, higher page numbers reach further into the past — for
// a chat view that loads older history as the user scrolls up rather than
// fetching the whole conversation up front. This is separate from what
// SendMessage feeds the agent (conversations.LoadHistory), which always
// needs the full conversation for context regardless of what's on screen.
//
//	@Summary		List a conversation's messages
//	@Description	Returns one page of a conversation's messages, newest first (page 1 is the most recent; higher page numbers reach further into the past) — for scroll-up-to-load-older-history. 404s for a conversation the caller doesn't own.
//	@Tags			chat
//	@Produce		json
//	@Param			id			path		int	true	"Conversation ID"
//	@Param			pageNumber	query		int	false	"Page number, default 1 (most recent)"
//	@Param			pageSize	query		int	false	"Page size, default 10"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[MessageResponse]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Failure		404			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id}/messages [get]
func (h *Handler) GetMessages(c *gin.Context) {
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

	var q listMessagesQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		shared.RespondValidationError(c, err)
		return
	}
	page := shared.Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}

	items, total, err := h.conversations.ListMessages(c.Request.Context(), userID, id, page)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	responses := make([]MessageResponse, 0, len(items))
	for _, msg := range items {
		responses = append(responses, toMessageResponse(msg))
	}

	shared.RespondSuccess(c, http.StatusOK, shared.NewPagedList(responses, total, page))
}

// SendMessage is the actual turn: load history, run the agent (with the
// caller's accessible agent-tool catalog registered, see buildRegistry),
// and stream the assistant's text back over SSE as it arrives instead of
// buffering the whole reply. The new user message and everything the
// agent produced are persisted only after the turn finishes.
//
//	@Summary		Send a message (SSE stream)
//	@Description	Sends a message and streams the reply as a Server-Sent Events response (Content-Type: text/event-stream) — this is NOT a plain JSON endpoint despite the shared.BaseResponse envelope every other route uses; it's documented here for completeness but tools like "Try it out" won't render it usefully. Events, in order: zero or more "chunk" (data is a raw string — one piece of assistant text as it streams in), then either "done" (data is []MessageResponse — every message this turn produced: the assistant's reply and any tool call/result pairs, already persisted) or "error" (data is a plain error string; nothing was persisted).
//	@Tags			chat
//	@Accept			json
//	@Produce		text/event-stream
//	@Param			id		path	int					true	"Conversation ID"
//	@Param			request	body	SendMessageRequest	true	"Message content"
//	@Success		200		{string}	string	"SSE stream — see description for event shapes"
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id}/messages [post]
func (h *Handler) SendMessage(c *gin.Context) {
	userID, ok := shared.GetUserID(c)
	if !ok {
		shared.AbortWithError(c, http.StatusUnauthorized, shared.ResultAuthError, errors.New("unauthenticated"))
		return
	}

	conversationID, err := shared.ParseIDParam(c)
	if err != nil {
		shared.RespondError(c, http.StatusBadRequest, shared.ResultValidationError, errors.New("invalid id"))
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	conv, err := h.conversations.Get(c.Request.Context(), userID, conversationID)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	llm, err := buildLLM(c.Request.Context(), h.providerKeys, h.customModels, userID, conv.Provider, conv.CustomModelID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	history, err := h.conversations.LoadHistory(c.Request.Context(), userID, conversationID)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	userMessage := ai_model.Message{Role: ai_model.RoleUser, Content: req.Content}
	turnInput := append(history, userMessage)
	originalLen := len(history)

	registry, err := buildRegistry(c.Request.Context(), userID, h.agentTools, h.sshconns, h.canAccess, h.commandPolicy)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	vohuAgent := agent.New(llm, registry, conv.Model)

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)
	flusher, canFlush := c.Writer.(http.Flusher)

	writeSSE := func(event string, payload any) {
		data, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		if canFlush {
			flusher.Flush()
		}
	}

	updated, err := vohuAgent.Run(c.Request.Context(), turnInput, func(chunk string) {
		writeSSE("chunk", chunk)
	})
	if err != nil {
		writeSSE("error", err.Error())
		return
	}

	newMessages := updated[originalLen:]
	if err := h.conversations.AppendHistory(c.Request.Context(), conversationID, newMessages); err != nil {
		writeSSE("error", fmt.Sprintf("reply was generated but failed to save: %v", err))
		return
	}

	responses := make([]MessageResponse, 0, len(newMessages))
	for _, msg := range newMessages {
		responses = append(responses, toMessageResponse(msg))
	}
	writeSSE("done", responses)
}
