package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Aliizi83/vohu/internal/agent"
	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/jobqueue"
	"github.com/Aliizi83/vohu/internal/platform/agenttool"
	"github.com/Aliizi83/vohu/internal/platform/commandrule"
	"github.com/Aliizi83/vohu/internal/platform/conversation"
	"github.com/Aliizi83/vohu/internal/platform/custommodel"
	"github.com/Aliizi83/vohu/internal/platform/customtool"
	"github.com/Aliizi83/vohu/internal/platform/providerkey"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/toolbuild"
	"github.com/gin-gonic/gin"
)

// Handler is the only place in internal/platform that imports
// internal/agent, internal/tools, internal/tools/command, and
// internal/ai_model/models (via llm.go) — every other platform module
// stays free of any dependency on Vohu's actual agent core.
type Handler struct {
	conversations  conversation.Service
	sshconns       sshconn.Service
	canAccess      shared.AccessLevelCheck
	commandRules   commandrule.Service
	providerKeys   providerkey.Service
	customModels   custommodel.Service
	agentTools     agenttool.Service
	customTools    customtool.Service
	jobs           jobqueue.Store
	defaultRetries int
	builder        toolbuild.Builder
}

func NewHandler(
	conversations conversation.Service,
	sshconns sshconn.Service,
	canAccess shared.AccessLevelCheck,
	commandRules commandrule.Service,
	providerKeys providerkey.Service,
	customModels custommodel.Service,
	agentTools agenttool.Service,
	customTools customtool.Service,
	jobs jobqueue.Store,
	defaultRetries int,
	builder toolbuild.Builder,
) *Handler {
	return &Handler{
		conversations:  conversations,
		sshconns:       sshconns,
		canAccess:      canAccess,
		commandRules:   commandRules,
		providerKeys:   providerKeys,
		customModels:   customModels,
		agentTools:     agentTools,
		customTools:    customTools,
		jobs:           jobs,
		defaultRetries: defaultRetries,
		builder:        builder,
	}
}

// CreateConversation starts a new conversation.
//
//	@Summary		Create a conversation
//	@Description	Starts a new conversation with the given provider/model — switchable later via PUT /conversations/{id}. customModelId, if set, names one of the caller's (or a global) custom model presets to use instead of the account's single per-provider key.
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
	userID, ok := shared.RequireUserID(c)
	if !ok {
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
	PageNumber int  `form:"pageNumber"`
	PageSize   int  `form:"pageSize"`
	Archived   bool `form:"archived"`
}

// ListConversations lists the caller's own conversations.
//
//	@Summary		List my conversations
//	@Description	Lists the caller's own conversations — archived=false (the default) for the normal list, archived=true for the archive view. There's no separate access-policy gate here beyond being authenticated — conversations are scoped to their owner by construction.
//	@Tags			chat
//	@Produce		json
//	@Param			pageNumber	query		int		false	"Page number, default 1"
//	@Param			pageSize	query		int		false	"Page size, default 10"
//	@Param			archived	query		bool	false	"List archived conversations instead of active ones, default false"
//	@Success		200			{object}	shared.BaseResponse{result=shared.PagedList[conversation.Response]}
//	@Failure		401			{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations [get]
func (h *Handler) ListConversations(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	var q listConversationsQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		shared.RespondValidationError(c, err)
		return
	}
	page := shared.Pagination{PageNumber: q.PageNumber, PageSize: q.PageSize}

	items, total, err := h.conversations.List(c.Request.Context(), userID, q.Archived, page)
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

// UpdateConversation renames and/or archives/unarchives a conversation —
// hand-written rather than shared.UpdateHandler because the access check
// (conversation.Service.Update) needs the caller's userID, which that
// generic's update func signature has no room for.
//
//	@Summary		Rename, archive/unarchive, or switch a conversation's model
//	@Description	Updates a conversation's title, archived flag, and/or which model it talks to going forward (provider+model+customModelId travel together as one unit — see conversation.UpdateConversationRequest). Omitting a field leaves it unchanged. Existing messages stay in history regardless of which model produced them. The caller must own the conversation, or hold "write" access to it (directly, or via its owner as a "user" resource).
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			id		path		int									true	"Conversation ID"
//	@Param			request	body		conversation.UpdateConversationRequest	true	"Fields to update"
//	@Success		200		{object}	shared.BaseResponse{result=conversation.Response}
//	@Failure		400		{object}	shared.BaseResponse
//	@Failure		401		{object}	shared.BaseResponse
//	@Failure		404		{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id} [put]
func (h *Handler) UpdateConversation(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	id, ok := shared.RequireIDParam(c)
	if !ok {
		return
	}

	var req conversation.UpdateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondValidationError(c, err)
		return
	}

	conv, err := h.conversations.Update(c.Request.Context(), userID, id, req)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, conversation.ToResponse(*conv))
}

// DeleteConversation permanently removes a conversation and its messages.
//
//	@Summary		Delete a conversation
//	@Description	Permanently deletes a conversation and every one of its messages. The caller must own the conversation, or hold "manage" access to it (directly, or via its owner as a "user" resource).
//	@Tags			chat
//	@Produce		json
//	@Param			id	path		int	true	"Conversation ID"
//	@Success		200	{object}	shared.BaseResponse
//	@Failure		401	{object}	shared.BaseResponse
//	@Failure		404	{object}	shared.BaseResponse
//	@Security		BearerAuth
//	@Router			/conversations/{id} [delete]
func (h *Handler) DeleteConversation(c *gin.Context) {
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	id, ok := shared.RequireIDParam(c)
	if !ok {
		return
	}

	if err := h.conversations.Delete(c.Request.Context(), userID, id); err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	shared.RespondSuccess(c, http.StatusOK, nil)
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
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	id, ok := shared.RequireIDParam(c)
	if !ok {
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
//	@Description	Sends a message and streams the reply as a Server-Sent Events response (Content-Type: text/event-stream) — this is NOT a plain JSON endpoint despite the shared.BaseResponse envelope every other route uses; it's documented here for completeness but tools like "Try it out" won't render it usefully. Events, in order: zero or more "chunk" (data is a raw string — one piece of assistant text as it streams in) interleaved with zero or more "tool_call" (data is an ai_model.ToolCall — fired the moment the model requests a call, before it runs) each eventually followed by its own "tool_result" (data is a ToolResultResponse, matched to its call by toolCallId) once that call finishes; an optional "title" (data is a plain string — only fired for a conversation's first message, once it's been renamed from the conversation's initial generic title to something derived from that message); then either "done" (data is []MessageResponse — every message this turn produced: the assistant's reply and any tool call/result pairs, already persisted) or "error" (data is a plain error string; nothing was persisted).
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
	userID, ok := shared.RequireUserID(c)
	if !ok {
		return
	}

	conversationID, ok := shared.RequireIDParam(c)
	if !ok {
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
	convSetting := conv.Setting

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

	registry, err := buildRegistry(c.Request.Context(), userID, h.agentTools, h.customTools, h.sshconns, h.canAccess, h.commandRules, h.jobs, h.defaultRetries, h.builder)
	if err != nil {
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	vohuAgent := agent.New(llm, registry, conv.Model, buildSystemPrompt(registry, convSetting.DefaultPrompt, int(convSetting.MaxToolIntegration)), int(convSetting.MaxToolIntegration))

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

	updated, err := vohuAgent.Run(c.Request.Context(), turnInput,
		func(chunk string) { writeSSE("chunk", chunk) },
		func(call ai_model.ToolCall) { writeSSE("tool_call", call) },
		func(result ai_model.ToolResult) { writeSSE("tool_result", toToolResultResponse(result)) },
	)
	if err != nil {
		writeSSE("error", err.Error())
		return
	}

	newMessages := updated[originalLen:]
	if err := h.conversations.AppendHistory(c.Request.Context(), conversationID, newMessages); err != nil {
		writeSSE("error", fmt.Sprintf("reply was generated but failed to save: %v", err))
		return
	}

	if len(history) == 0 {
		title := deriveTitle(req.Content)
		if _, err := h.conversations.Update(c.Request.Context(), userID, conversationID, conversation.UpdateConversationRequest{Title: title}); err == nil {
			writeSSE("title", title)
		}
	}

	responses := make([]MessageResponse, 0, len(newMessages))
	for _, msg := range newMessages {
		responses = append(responses, toMessageResponse(msg))
	}
	writeSSE("done", responses)
}

const maxDerivedTitleLength = 60

func deriveTitle(content string) string {
	collapsed := strings.Join(strings.Fields(content), " ")
	if collapsed == "" {
		return "New chat"
	}

	runes := []rune(collapsed)
	if len(runes) <= maxDerivedTitleLength {
		return collapsed
	}
	return string(runes[:maxDerivedTitleLength]) + "…"
}
