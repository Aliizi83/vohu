package chat_settings

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, s *ChatSetting) error
	FindByConversationID(ctx context.Context, conversationID uint) (*ChatSetting, error)
	Update(ctx context.Context, s *ChatSetting) error
	WithTx(tx *gorm.DB) Repository
}

type gormRepository struct {
	generic *shared.GenericRepository[ChatSetting]
	db      *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		generic: shared.NewGenericRepository[ChatSetting](db),
		db:      db,
	}
}

func (r *gormRepository) Create(ctx context.Context, s *ChatSetting) error {
	return r.generic.Create(ctx, s)
}

func (r *gormRepository) FindByConversationID(ctx context.Context, conversationID uint) (*ChatSetting, error) {
	var row ChatSetting
	err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *gormRepository) Update(ctx context.Context, s *ChatSetting) error {
	return r.generic.Update(ctx, s)
}

func (r *gormRepository) WithTx(tx *gorm.DB) Repository {
	return NewRepository(tx)
}
