package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V3_DropTripGroupID removes the standing-group foreign key.
// A trip no longer belongs to a group. AutoMigrate does not drop columns on its own.
func V3_DropTripGroupID(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("trips") {
		return nil
	}
	var exists bool
	err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'trips'
			  AND column_name = 'group_id'
		)
	`).Scan(&exists).Error
	if err != nil {
		return fmt.Errorf("inspect trips.group_id: %w", err)
	}
	if !exists {
		return nil
	}
	if err := db.Exec(`ALTER TABLE trips DROP COLUMN IF EXISTS group_id CASCADE`).Error; err != nil {
		return fmt.Errorf("drop trips.group_id: %w", err)
	}
	return nil
}
