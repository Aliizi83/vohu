package conversation

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/chat_settings"
	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var ErrNotOwner = errors.New("conversation does not belong to this user")

const resourceTypeConversation = "conversation"
const defaultTitle = "New chat"

type Service interface {
	Create(ctx context.Context, userID uint, req CreateConversationRequest) (*Conversation, error)
	Get(ctx context.Context, userID uint, id uint) (*Conversation, error)
	List(ctx context.Context, userID uint, archived bool, page shared.Pagination) ([]Conversation, int64, error)
	Update(ctx context.Context, userID uint, id uint, req UpdateConversationRequest) (*Conversation, error)
	Delete(ctx context.Context, userID uint, id uint) error
	LoadHistory(ctx context.Context, userID uint, conversationID uint) ([]ai_model.Message, error)
	ListMessages(ctx context.Context, userID uint, conversationID uint, page shared.Pagination) ([]ai_model.Message, int64, error)
	AppendHistory(ctx context.Context, conversationID uint, messages []ai_model.Message) error
}

type service struct {
	repo             Repository
	chatSettingsRepo chat_settings.Repository
	hasAccessLevel   shared.AccessLevelCheck
}

func NewService(repo Repository, chatSettingsRepo chat_settings.Repository, hasAccessLevel shared.AccessLevelCheck) Service {
	return &service{repo: repo, chatSettingsRepo: chatSettingsRepo, hasAccessLevel: hasAccessLevel}
}

func (s *service) Create(ctx context.Context, userID uint, req CreateConversationRequest) (*Conversation, error) {
	title := req.Title
	if title == "" {
		title = defaultTitle
	}

	conv := &Conversation{
		UserID:        userID,
		Title:         title,
		Provider:      req.Provider,
		Model:         req.Model,
		CustomModelID: req.CustomModelID,
	}

	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}

	if err := s.chatSettingsRepo.Create(ctx, &chat_settings.ChatSetting{
		ConversationID: conv.ID,
	}); err != nil {
		return nil, err
	}

	return conv, nil
}

func (s *service) Get(ctx context.Context, userID uint, id uint) (*Conversation, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil {
		return nil, err
	}

	allowed, err := s.canAccess(ctx, userID, conv, "read")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, shared.ErrNotFound
	}

	return conv, nil
}

// canAccess is Get's original owner-or-grant check, generalized to any
// level — Update reuses it at "write," Delete at "manage." Not the
// owner — two independent paths can still grant access:
//
//  1. A grant directly on this conversation (resourceType
//     "conversation") — for sharing one specific thread.
//  2. A grant on the conversation's *owner* as a "user" resource —
//     this is what makes "a support role can manage every regular
//     user's conversations" actually work: rbac.Service.HasAccessLevel
//     already cascades a role-level grant on resourceType "role" to
//     every user holding that role (see its doc comment), so checking
//     "user"/conv.UserID here reuses that cascade instead of needing
//     a duplicate one for conversations specifically. A per-user
//     prohibited row (the same mechanism that carves a user out of
//     the cascade for their own profile) carves them out of this too.
func (s *service) canAccess(ctx context.Context, userID uint, conv *Conversation, level string) (bool, error) {
	if conv.UserID == userID {
		return true, nil
	}

	allowedDirect, err := s.hasAccessLevel(ctx, userID, resourceTypeConversation, conv.ID, level)
	if err != nil {
		return false, err
	}
	if allowedDirect {
		return true, nil
	}

	return s.hasAccessLevel(ctx, userID, "user", conv.UserID, level)
}

func (s *service) List(
	ctx context.Context,
	userID uint,
	archived bool,
	page shared.Pagination,
) ([]Conversation, int64, error) {
	return s.repo.ListConversationsByUser(ctx, userID, archived, page)
}

func (s *service) Update(
	ctx context.Context,
	userID uint,
	id uint,
	req UpdateConversationRequest,
) (*Conversation, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil {
		return nil, err
	}

	allowed, err := s.canAccess(ctx, userID, conv, "write")
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, shared.ErrNotFound
	}

	if req.Title != "" {
		conv.Title = req.Title
	}
	if req.Archived != nil {
		conv.Archived = *req.Archived
	}
	if req.Provider != "" {
		conv.Provider = req.Provider
		conv.Model = req.Model
		conv.CustomModelID = req.CustomModelID
	}

	if err := s.repo.UpdateConversation(ctx, conv); err != nil {
		return nil, err
	}

	return conv, nil
}

func (s *service) Delete(ctx context.Context, userID uint, id uint) error {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil {
		return err
	}

	allowed, err := s.canAccess(ctx, userID, conv, "manage")
	if err != nil {
		return err
	}
	if !allowed {
		return shared.ErrNotFound
	}

	return s.repo.DeleteConversation(ctx, id)
}

func (s *service) LoadHistory(ctx context.Context, userID uint, conversationID uint) ([]ai_model.Message, error) {
	if _, err := s.Get(ctx, userID, conversationID); err != nil {
		return nil, err
	}

	rows, err := s.repo.ListMessages(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	messages := make([]ai_model.Message, 0, len(rows))
	for _, row := range rows {
		msg, err := toAgentMessage(row)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *service) ListMessages(
	ctx context.Context, userID uint, conversationID uint, page shared.Pagination,
) ([]ai_model.Message, int64, error) {
	if _, err := s.Get(ctx, userID, conversationID); err != nil {
		return nil, 0, err
	}

	rows, total, err := s.repo.ListMessagesPage(ctx, conversationID, page)
	if err != nil {
		return nil, 0, err
	}

	messages := make([]ai_model.Message, 0, len(rows))
	for _, row := range rows {
		msg, err := toAgentMessage(row)
		if err != nil {
			return nil, 0, err
		}
		messages = append(messages, msg)
	}

	return messages, total, nil
}

func (s *service) AppendHistory(ctx context.Context, conversationID uint, messages []ai_model.Message) error {
	rows := make([]Message, 0, len(messages))
	for _, msg := range messages {
		row, err := toEntityMessage(conversationID, msg)
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}

	return s.repo.AppendMessages(ctx, rows)
}
