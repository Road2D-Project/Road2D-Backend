package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V6_AddUserNumberPhone fills number_phone on existing users before AutoMigrate
// marks the column NOT NULL. A new database has no users table yet, so this
// step returns and AutoMigrate creates the column from the model.
func V6_AddUserNumberPhone(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("users") {
		return nil
	}
	statements := []string{
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS number_phone varchar(255)`,
		`UPDATE users SET number_phone = '' WHERE number_phone IS NULL`,
		`ALTER TABLE users ALTER COLUMN number_phone SET NOT NULL`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("user number_phone: %w", err)
		}
	}
	return nil
}
