package user

import (
	"context"
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/shared"
	"gorm.io/gorm"
)

type Repository interface {
	Create(ctx context.Context, u *User) error
	FindByID(ctx context.Context, id uint) (*User, error)
	Update(ctx context.Context, u *User) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, filter shared.DynamicFilter, page shared.Pagination) ([]User, int64, error)

	FindByUsername(ctx context.Context, username string) (*User, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
}

// gormRepository embeds the generic repository for plain CRUD (Create,
// FindByID, Update, Delete, List come from there — see
// internal/platform/shared/repository.go) and adds the lookups that are
// more than CRUD.
type gormRepository struct {
	*shared.GenericRepository[User]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{
		GenericRepository: shared.NewGenericRepository[User](db),
		db:                db,
	}
}

func (r *gormRepository) FindByUsername(ctx context.Context, username string) (*User, error) {
	var u User

	err := r.db.WithContext(ctx).Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, shared.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

func (r *gormRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&User{}).Where("username = ?", username).Count(&count).Error
	return count > 0, err
}
