package configuration

import (
	"fmt"
	"log"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := ensureUsersIDIsUUID(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&model.PlaceType{},
		&model.Location{},
		&model.Destination{},
		&model.Trip{},
		&model.TripBranch{},
		&model.BranchDestination{},
		&model.Leg{},
		&model.Travel{},
		&authModel.User{},
		&authModel.RevokedRefreshToken{},
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
