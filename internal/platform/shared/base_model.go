package shared

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel is the common set of audit columns every platform entity
// embeds. Adapted from sample-golang-project's domain/models/base_model.go,
// trimmed: no random Code field, and soft-delete uses GORM's own
// gorm.DeletedAt so queries filter deleted rows automatically.
type BaseModel struct {
	ID        uint           `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
