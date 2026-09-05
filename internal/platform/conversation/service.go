package conversation

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var ErrNotOwner = errors.New("conversation does not belong to this user")

type Service interface {
	Create(ctx context.Context, userID uint, req CreateConversationRequest) (*Conversation, error)

	// Get returns shared.ErrNotFound both when the conversation truly
	// doesn't exist and when it exists but belongs to someone else — a
	// caller has no legitimate reason to distinguish "not found" from
	// "not yours."
	Get(ctx context.Context, userID uint, id uint) (*Conversation, error)
	List(ctx context.Context, userID uint, page shared.Pagination) ([]Conversation, int64, error)

	// LoadHistory returns a conversation's messages translated into the
	// shape agent.Agent.Run takes directly.
	LoadHistory(ctx context.Context, userID uint, conversationID uint) ([]ai_model.Message, error)

	// AppendHistory persists new messages produced by one agent turn —
	// typically everything Agent.Run returned beyond what LoadHistory
	// handed it.
	AppendHistory(ctx context.Context, conversationID uint, messages []ai_model.Message) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Create(ctx context.Context, userID uint, req CreateConversationRequest) (*Conversation, error) {
	conv := &Conversation{
		UserID:   userID,
		Title:    req.Title,
		Provider: req.Provider,
		Model:    req.Model,
	}

	if err := s.repo.CreateConversation(ctx, conv); err != nil {
		return nil, err
	}

	return conv, nil
}

func (s *service) Get(ctx context.Context, userID uint, id uint) (*Conversation, error) {
	conv, err := s.repo.FindConversationByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if conv.UserID != userID {
		return nil, shared.ErrNotFound
	}

	return conv, nil
}

func (s *service) List(
	ctx context.Context,
	userID uint,
	page shared.Pagination,
) ([]Conversation, int64, error) {
	return s.repo.ListConversationsByUser(ctx, userID, page)
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
