package conversation

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/ai_model"
	"github.com/Aliizi83/vohu/internal/platform/shared"
)

var ErrNotOwner = errors.New("conversation does not belong to this user")

const resourceTypeConversation = "conversation"

type Service interface {
	Create(ctx context.Context, userID uint, req CreateConversationRequest) (*Conversation, error)

	// Get returns shared.ErrNotFound both when the conversation truly
	// doesn't exist and when it exists but belongs to someone else *and*
	// the caller holds no resource-level access to it either (see
	// hasAccessLevel) — a caller has no legitimate reason to distinguish
	// "not found" from "not yours and not shared with you."
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
	repo           Repository
	hasAccessLevel shared.AccessLevelCheck
}

func NewService(repo Repository, hasAccessLevel shared.AccessLevelCheck) Service {
	return &service{repo: repo, hasAccessLevel: hasAccessLevel}
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

	if conv.UserID == userID {
		return conv, nil
	}

	// Not the owner — two independent paths can still grant access:
	//
	//  1. A grant directly on this conversation (resourceType
	//     "conversation") — for sharing one specific thread.
	//  2. A grant on the conversation's *owner* as a "user" resource —
	//     this is what makes "a support role can read every regular
	//     user's conversations" actually work: rbac.Service.HasAccessLevel
	//     already cascades a role-level grant on resourceType "role" to
	//     every user holding that role (see its doc comment), so checking
	//     "user"/conv.UserID here reuses that cascade instead of needing
	//     a duplicate one for conversations specifically. A per-user
	//     prohibited row (the same mechanism that carves a user out of
	//     the cascade for their own profile) carves them out of this too.
	//
	// Still default-deny overall: neither path granting means
	// shared.ErrNotFound, same as if the conversation didn't exist.
	allowedDirect, err := s.hasAccessLevel(ctx, userID, resourceTypeConversation, conv.ID, "read")
	if err != nil {
		return nil, err
	}
	if allowedDirect {
		return conv, nil
	}

	allowedViaOwner, err := s.hasAccessLevel(ctx, userID, "user", conv.UserID, "read")
	if err != nil {
		return nil, err
	}
	if !allowedViaOwner {
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
