package version

import (
	"fmt"

	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

// V2_DropUniqueLocationNameIndex removes the old unique name index.
// Seeded Goong results often share a display name; identity is place_id.
func V2_DropUniqueLocationNameIndex(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("locations") {
		return nil
	}
	if !db.Migrator().HasIndex(&model.Location{}, "idx_location_name") {
		return nil
	}
	if err := db.Migrator().DropIndex(&model.Location{}, "idx_location_name"); err != nil {
		return fmt.Errorf("drop idx_location_name: %w", err)
	}
	return nil
}
