package shared

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrNotFound is the one sentinel every module's FindByID/Delete returns —
// consolidated here instead of each module defining its own, so handlers
// across modules check the same error.
var ErrNotFound = errors.New("record not found")

// GenericRepository provides the plain-CRUD operations any GORM-backed
// entity needs. Modules embed this for Create/FindByID/Update/Delete/List
// and add their own repository methods for anything beyond that (lookups
// by a non-ID field, joins, aggregate queries, ...).
type GenericRepository[T any] struct {
	DB *gorm.DB
}

func NewGenericRepository[T any](db *gorm.DB) *GenericRepository[T] {
	return &GenericRepository[T]{DB: db}
}

func (r *GenericRepository[T]) Create(ctx context.Context, model *T) error {
	return r.DB.WithContext(ctx).Create(model).Error
}

func (r *GenericRepository[T]) FindByID(ctx context.Context, id uint) (*T, error) {
	var model T

	err := r.DB.WithContext(ctx).First(&model, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &model, nil
}

func (r *GenericRepository[T]) Update(ctx context.Context, model *T) error {
	return r.DB.WithContext(ctx).Save(model).Error
}

func (r *GenericRepository[T]) Delete(ctx context.Context, id uint) error {
	var model T

	result := r.DB.WithContext(ctx).Delete(&model, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (r *GenericRepository[T]) List(
	ctx context.Context,
	filter DynamicFilter,
	page Pagination,
) ([]T, int64, error) {

	var items []T
	var total int64

	query, err := ApplyDynamicFilter[T](r.DB.WithContext(ctx), filter)
	if err != nil {
		return nil, 0, err
	}

	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Model(new(T)).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	sortQuery, err := ApplySort[T](query, filter.Sorts)
	if err != nil {
		return nil, 0, err
	}

	err = sortQuery.
		Offset(page.Offset()).
		Limit(page.Limit()).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}
