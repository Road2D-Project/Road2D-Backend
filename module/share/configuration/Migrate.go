package configuration

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	groupModel "Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/share/configuration/version"
	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

// AutoMigrate runs versioned repairs, then aligns tables with the current models.
// The version list and the rules live in configuration/version.
func AutoMigrate(db *gorm.DB) error {
	if err := version.Apply(db); err != nil {
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
