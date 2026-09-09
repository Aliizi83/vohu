package custommodel

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, m *CustomModel) error
	FindByID(ctx context.Context, id uint) (*CustomModel, error)
	ListForUser(ctx context.Context, userID uint) ([]CustomModel, error)
	ListGlobal(ctx context.Context) ([]CustomModel, error)

	// DeleteForUser/DeleteGlobal both scope the delete to the expected
	// ownership (WHERE user_id = ? / WHERE user_id IS NULL) in the same
	// query as the delete itself, so a user can never delete another
	// user's personal preset or a global one by guessing its ID.
	DeleteForUser(ctx context.Context, userID uint, id uint) error
	DeleteGlobal(ctx context.Context, id uint) error
}

type gormRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) Create(ctx context.Context, m *CustomModel) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *gormRepository) FindByID(ctx context.Context, id uint) (*CustomModel, error) {
	var m CustomModel
	err := r.db.WithContext(ctx).First(&m, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *gormRepository) ListForUser(ctx context.Context, userID uint) ([]CustomModel, error) {
	var models []CustomModel
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("name ASC").Find(&models).Error
	return models, err
}

func (r *gormRepository) ListGlobal(ctx context.Context) ([]CustomModel, error) {
	var models []CustomModel
	err := r.db.WithContext(ctx).Where("user_id IS NULL").Order("name ASC").Find(&models).Error
	return models, err
}

func (r *gormRepository) DeleteForUser(ctx context.Context, userID uint, id uint) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).Delete(&CustomModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}

func (r *gormRepository) DeleteGlobal(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Where("id = ? AND user_id IS NULL", id).Delete(&CustomModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return shared.ErrNotFound
	}
	return nil
}
