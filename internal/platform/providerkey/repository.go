package providerkey

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	FindGlobal(ctx context.Context, provider Provider) (*Key, error)
	FindForUser(ctx context.Context, userID uint, provider Provider) (*Key, error)
	ListGlobal(ctx context.Context) ([]Key, error)
	ListForUser(ctx context.Context, userID uint) ([]Key, error)

	UpsertGlobal(ctx context.Context, provider Provider, encryptedAPIKey, baseURL, workspaceID string) error
	UpsertForUser(ctx context.Context, userID uint, provider Provider, encryptedAPIKey, baseURL, workspaceID string) error

	DeleteGlobal(ctx context.Context, provider Provider) error
	DeleteForUser(ctx context.Context, userID uint, provider Provider) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindGlobal(ctx context.Context, provider Provider) (*Key, error) {
	var key Key

	err := r.db.WithContext(ctx).
		Where("user_id IS NULL AND provider = ?", provider).
		First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (r *gormRepository) FindForUser(ctx context.Context, userID uint, provider Provider) (*Key, error) {
	var key Key

	err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		First(&key).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &key, nil
}

func (r *gormRepository) ListGlobal(ctx context.Context) ([]Key, error) {
	var keys []Key
	err := r.db.WithContext(ctx).Where("user_id IS NULL").Order("provider ASC").Find(&keys).Error
	return keys, err
}

func (r *gormRepository) ListForUser(ctx context.Context, userID uint) ([]Key, error) {
	var keys []Key
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("provider ASC").Find(&keys).Error
	return keys, err
}

func (r *gormRepository) UpsertGlobal(
	ctx context.Context,
	provider Provider,
	encryptedAPIKey, baseURL, workspaceID string,
) error {
	existing, err := r.FindGlobal(ctx, provider)
	if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return err
	}

	if existing != nil {
		existing.EncryptedAPIKey = encryptedAPIKey
		existing.BaseURL = baseURL
		existing.WorkspaceID = workspaceID
		return r.db.WithContext(ctx).Save(existing).Error
	}

	return r.db.WithContext(ctx).Create(&Key{
		UserID:          nil,
		Provider:        provider,
		EncryptedAPIKey: encryptedAPIKey,
		BaseURL:         baseURL,
		WorkspaceID:     workspaceID,
	}).Error
}

func (r *gormRepository) UpsertForUser(
	ctx context.Context,
	userID uint,
	provider Provider,
	encryptedAPIKey, baseURL, workspaceID string,
) error {
	existing, err := r.FindForUser(ctx, userID, provider)
	if err != nil && !errors.Is(err, shared.ErrNotFound) {
		return err
	}

	if existing != nil {
		existing.EncryptedAPIKey = encryptedAPIKey
		existing.BaseURL = baseURL
		existing.WorkspaceID = workspaceID
		return r.db.WithContext(ctx).Save(existing).Error
	}

	return r.db.WithContext(ctx).Create(&Key{
		UserID:          &userID,
		Provider:        provider,
		EncryptedAPIKey: encryptedAPIKey,
		BaseURL:         baseURL,
		WorkspaceID:     workspaceID,
	}).Error
}

func (r *gormRepository) DeleteGlobal(ctx context.Context, provider Provider) error {
	result := r.db.WithContext(ctx).
		Where("user_id IS NULL AND provider = ?", provider).
		Delete(&Key{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *gormRepository) DeleteForUser(ctx context.Context, userID uint, provider Provider) error {
	result := r.db.WithContext(ctx).
		Where("user_id = ? AND provider = ?", userID, provider).
		Delete(&Key{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
