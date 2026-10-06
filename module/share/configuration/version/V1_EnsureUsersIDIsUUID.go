package version

import (
	"fmt"
	"log"

	"gorm.io/gorm"
)

// V1_EnsureUsersIDIsUUID recreates users when id is still bigint.
// GORM AutoMigrate does not change an existing column type, and the model uses uuid.
func V1_EnsureUsersIDIsUUID(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("users") {
		return nil
	}

	var dataType string
	err := db.Raw(`
		SELECT data_type
		FROM information_schema.columns
		WHERE table_schema = current_schema()
		  AND table_name = 'users'
		  AND column_name = 'id'
	`).Scan(&dataType).Error
	if err != nil {
		return fmt.Errorf("inspect users.id: %w", err)
	}
	if dataType == "" || dataType == "uuid" {
		return nil
	}

	log.Printf("users.id is %s; recreating users table as uuid", dataType)
	if err := db.Migrator().DropTable("users"); err != nil {
		return fmt.Errorf("drop users: %w", err)
	}
	return nil
}
