package conversation

import (
	"context"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	CreateConversation(ctx context.Context, c *Conversation) error
	FindConversationByID(ctx context.Context, id uint) (*Conversation, error)
	ListConversationsByUser(ctx context.Context, userID uint, archived bool, page shared.Pagination) ([]Conversation, int64, error)
	UpdateConversation(ctx context.Context, c *Conversation) error
	DeleteConversation(ctx context.Context, id uint) error
	AppendMessages(ctx context.Context, rows []Message) error
	ListMessages(ctx context.Context, conversationID uint) ([]Message, error)
	ListMessagesPage(ctx context.Context, conversationID uint, page shared.Pagination) ([]Message, int64, error)
	WithTx(tx *gorm.DB) Repository
}

type gormRepository struct {
	db            *gorm.DB
	conversations *shared.GenericRepository[Conversation]
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		db:            db,
		conversations: shared.NewGenericRepository[Conversation](db),
	}
}

func (r *gormRepository) CreateConversation(ctx context.Context, c *Conversation) error {
	return r.conversations.Create(ctx, c)
}

func (r *gormRepository) FindConversationByID(ctx context.Context, id uint) (*Conversation, error) {
	return r.conversations.FindByID(ctx, id)
}

// ListConversationsByUser is hand-written rather than going through
// shared.ApplyDynamicFilter — that helper's FieldFilter only builds
// string/ILIKE comparisons (it exists for user-supplied filter UIs), not
// an exact numeric equality on a foreign key that every caller of this
// method needs unconditionally.
func (r *gormRepository) ListConversationsByUser(
	ctx context.Context,
	userID uint,
	archived bool,
	page shared.Pagination,
) ([]Conversation, int64, error) {
	var items []Conversation
	var total int64

	query := r.db.WithContext(ctx).Model(&Conversation{}).
		Where("user_id = ? AND archived = ?", userID, archived)

	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func (r *gormRepository) UpdateConversation(ctx context.Context, c *Conversation) error {
	return r.conversations.Update(ctx, c)
}

func (r *gormRepository) DeleteConversation(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Where("conversation_id = ?", id).Delete(&Message{}).Error; err != nil {
		return err
	}
	return r.conversations.Delete(ctx, id)
}

func (r *gormRepository) WithTx(tx *gorm.DB) Repository {
	return NewRepository(tx)
}

func (r *gormRepository) AppendMessages(ctx context.Context, rows []Message) error {
	if len(rows) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&rows).Error
}

func (r *gormRepository) ListMessages(ctx context.Context, conversationID uint) ([]Message, error) {
	var rows []Message

	err := r.db.WithContext(ctx).
		Where("conversation_id = ?", conversationID).
		Order("id ASC").
		Find(&rows).Error

	return rows, err
}

// ListMessagesPage fetches the Nth most-recent chunk (page 1 = newest)
// via ORDER BY id DESC + offset/limit, the same pattern
// ListConversationsByUser uses, then reverses that chunk back to
// chronological order — callers want to render a page top-to-bottom like
// the rest of the conversation, "newest first" is only how pages are
// numbered, not how a single page reads.
func (r *gormRepository) ListMessagesPage(
	ctx context.Context,
	conversationID uint,
	page shared.Pagination,
) ([]Message, int64, error) {
	var items []Message
	var total int64

	query := r.db.WithContext(ctx).Model(&Message{}).Where("conversation_id = ?", conversationID)

	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Order("id DESC").
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}

	return items, total, nil
}
