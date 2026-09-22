package chat_settings

import (
	"context"
)

type UpdateChatSettingRequest struct {
	DefaultToolIntegration uint
	DefaultPrompt          string `binding:"max=8000"`
}

type Service interface {
	Get(ctx context.Context, conversationID uint) (*ChatSetting, error)
	Upsert(ctx context.Context, conversationID uint, req UpdateChatSettingRequest) (*ChatSetting, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Get(ctx context.Context, conversationID uint) (*ChatSetting, error) {
	setting, err := s.repo.FindByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *service) Upsert(ctx context.Context, conversationID uint, req UpdateChatSettingRequest) (*ChatSetting, error) {
	existing, err := s.repo.FindByConversationID(ctx, conversationID)
	if err != nil {
		return nil, err
	}

	existing.MaxToolIntegration = req.DefaultToolIntegration
	existing.DefaultPrompt = req.DefaultPrompt
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	return existing, nil
}
