package user

import "github.com/Aliizi83/vohu/internal/platform/shared"

type User struct {
	shared.BaseModel
	Username string `gorm:"type:varchar(50);not null;unique"`
	Email    string `gorm:"type:varchar(100)"`
	Password string `gorm:"type:varchar(255);not null"`
	Enabled  bool   `gorm:"not null;default:true"`
}

func (User) TableName() string {
	return "users"
}

func init() {
	shared.RegisterModel(&User{})
}
