package configuration

import (
	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.PlaceType{},
		&model.Location{},
		&model.Destination{},
		&model.Trip{},
		&model.TripBranch{},
		&model.BranchDestination{},
		&model.Leg{},
		&model.Travel{},
	)
}
