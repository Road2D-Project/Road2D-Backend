package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V4_AddTripPolicyColumns gives existing trips a bronze cap and private visibility
// before AutoMigrate marks the columns NOT NULL. A new database has no trips
// table yet, so this step returns and AutoMigrate creates the columns from the model.
func V4_AddTripPolicyColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("trips") {
		return nil
	}
	statements := []string{
		`ALTER TABLE trips ADD COLUMN IF NOT EXISTS trip_type varchar(16)`,
		`UPDATE trips SET trip_type = 'bronze' WHERE trip_type IS NULL OR trip_type = ''`,
		`ALTER TABLE trips ALTER COLUMN trip_type SET DEFAULT 'bronze'`,
		`ALTER TABLE trips ALTER COLUMN trip_type SET NOT NULL`,
		`ALTER TABLE trips ADD COLUMN IF NOT EXISTS member_limit integer`,
		`UPDATE trips SET member_limit = 15 WHERE member_limit IS NULL`,
		`ALTER TABLE trips ALTER COLUMN member_limit SET DEFAULT 15`,
		`ALTER TABLE trips ALTER COLUMN member_limit SET NOT NULL`,
		`ALTER TABLE trips ADD COLUMN IF NOT EXISTS visibility boolean`,
		`UPDATE trips SET visibility = false WHERE visibility IS NULL`,
		`ALTER TABLE trips ALTER COLUMN visibility SET DEFAULT false`,
		`ALTER TABLE trips ALTER COLUMN visibility SET NOT NULL`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("trip policy columns: %w", err)
		}
	}
	return nil
}
