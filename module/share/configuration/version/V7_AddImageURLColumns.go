package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V7_AddImageURLColumns adds nullable image URLs before AutoMigrate.
// A new database has no tables yet, so this step returns and AutoMigrate
// creates the columns from the models.
func V7_AddImageURLColumns(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	if db.Migrator().HasTable("users") {
		if err := db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url text`).Error; err != nil {
			return fmt.Errorf("user avatar_url: %w", err)
		}
	}
	if db.Migrator().HasTable("trips") {
		if err := db.Exec(`ALTER TABLE trips ADD COLUMN IF NOT EXISTS image_url text`).Error; err != nil {
			return fmt.Errorf("trip image_url: %w", err)
		}
	}
	return nil
}
