package seeders

import (
	"errors"

	"github.com/Aliizi83/vohu/internal/platform/user"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Dev-only default credentials, same pattern as sample-golang-project's
// constants.DefaultUserUsername/.../DefaultUserPassword. Change these (or
// move to config) before this is ever exposed beyond localhost.
const (
	DefaultAdminUsername = "admin"
	DefaultAdminEmail    = "admin@example.com"
	DefaultAdminPassword = "change-me-now"
)

func seedUsers(database *gorm.DB) error {
	var existing user.User

	err := database.Where("username = ?", DefaultAdminUsername).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(DefaultAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := &user.User{
		Username: DefaultAdminUsername,
		Email:    DefaultAdminEmail,
		Password: string(hashed),
		Enabled:  true,
	}

	return database.Create(admin).Error
}
