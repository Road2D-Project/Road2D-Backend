package configuration

import (
	"fmt"
	"log"

	authModel "Road-To-Destination-BE/module/authentication/model"
	groupModel "Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := ensureUsersIDIsUUID(db); err != nil {
		return err
	}
	if err := dropUniqueLocationNameIndex(db); err != nil {
		return err
	}
	if err := dropTripGroupID(db); err != nil {
		return err
	}
	if err := addTripPolicyColumns(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&authModel.User{},
		&authModel.RevokedRefreshToken{},
		&groupModel.Group{},
		&groupModel.GroupMember{},
		&model.PlaceType{},
		&model.Location{},
		&model.Destination{},
		&model.Trip{},
		&model.TripMember{},
		&model.TripBranch{},
		&model.BranchDestination{},
		&model.Leg{},
		&model.Travel{},
	)
}

// GORM AutoMigrate does not change an existing column type. users.id was
// leftover as bigint while the model uses uuid.
func ensureUsersIDIsUUID(db *gorm.DB) error {
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

// dropUniqueLocationNameIndex removes the old unique name index. Seeded Goong
// results often share a display name; identity is place_id.
func dropUniqueLocationNameIndex(db *gorm.DB) error {
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

// dropTripGroupID removes the standing-group foreign key. A trip no longer
// belongs to a group; AutoMigrate does not drop columns on its own.
func dropTripGroupID(db *gorm.DB) error {
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

// addTripPolicyColumns gives existing trips a bronze cap and private visibility
// before AutoMigrate marks the columns NOT NULL. New databases skip this and
// let AutoMigrate create the columns with the model.
func addTripPolicyColumns(db *gorm.DB) error {
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
