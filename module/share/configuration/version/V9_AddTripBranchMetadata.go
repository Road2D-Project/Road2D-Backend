package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V9_AddTripBranchMetadata adds description and color on trip branches.
// A new database has no table yet, so this step returns and AutoMigrate
// creates the columns from the model. Existing rows get an empty string.
func V9_AddTripBranchMetadata(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("trip_branches") {
		return nil
	}
	statements := []string{
		`ALTER TABLE trip_branches ADD COLUMN IF NOT EXISTS description text NOT NULL DEFAULT ''`,
		`ALTER TABLE trip_branches ADD COLUMN IF NOT EXISTS color varchar(7) NOT NULL DEFAULT ''`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("trip branch metadata: %w", err)
		}
	}
	return nil
}
