package chat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Aliizi83/vohu/internal/agent"
	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/conversation"
	"github.com/Aliizi83/vohu/internal/platform/providerkey"
	"github.com/Aliizi83/vohu/internal/platform/shared"
	"github.com/Aliizi83/vohu/internal/platform/sshconn"
	"github.com/Aliizi83/vohu/internal/tools"
	"github.com/Aliizi83/vohu/internal/tools/command"
	"github.com/Aliizi83/vohu/internal/tools/system_tools"
	"github.com/gin-gonic/gin"
)

// Handler is the only place in internal/platform that imports
// internal/agent, internal/tools, internal/tools/command, and
// internal/ai_model/models (via llm.go) — every other platform module
// stays free of any dependency on Vohu's actual agent core.
type Handler struct {
	conversations conversation.Service
	sshconns      sshconn.Service
	canAccess     CanAccessResource
	commandPolicy command.Policy
	providerKeys  providerkey.Service
}

func NewHandler(
	conversations conversation.Service,
	sshconns sshconn.Service,
	canAccess CanAccessResource,
	commandPolicy command.Policy,
	providerKeys providerkey.Service,
) *Handler {
	return &Handler{
		conversations: conversations,
		sshconns:      sshconns,
		canAccess:     canAccess,
		commandPolicy: commandPolicy,
		providerKeys:  providerKeys,
	}
}

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

	history, err := h.conversations.LoadHistory(c.Request.Context(), userID, id)
	if err != nil {
		if errors.Is(err, shared.ErrNotFound) {
			shared.RespondError(c, http.StatusNotFound, shared.ResultNotFoundError, err)
			return
		}
		shared.RespondError(c, http.StatusInternalServerError, shared.ResultInternalError, err)
		return
	}

	responses := make([]MessageResponse, 0, len(history))
	for _, msg := range history {
		responses = append(responses, toMessageResponse(msg))
	}

	shared.RespondSuccess(c, http.StatusOK, responses)
}

// SendMessage is the actual turn: load history, run the agent (with the
// system-time tool and this request's SSHTool registered), and stream the
// assistant's text back over SSE as it arrives instead of buffering the
// whole reply. The new user message and everything the agent produced are
// persisted only after the turn finishes.
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

	llm, err := buildLLM(c.Request.Context(), h.providerKeys, userID, conv.Provider)
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

	registry := tools.NewRegistry()
	registry.Register(system_tools.NewCurrentSystemTime())
	registry.Register(NewListSSHConnectionsTool(userID, h.sshconns, h.canAccess))
	registry.Register(NewSSHTool(userID, h.sshconns, h.canAccess, h.commandPolicy))

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
