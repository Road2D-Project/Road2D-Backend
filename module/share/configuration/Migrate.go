package configuration

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	chatModel "Road-To-Destination-BE/module/chat/model"
	friendModel "Road-To-Destination-BE/module/friend/model"
	groupModel "Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/share/configuration/version"
	"Road-To-Destination-BE/module/share/outbox"
	trackingModel "Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/trip/model"

	"gorm.io/gorm"
)

// AutoMigrate runs versioned repairs, then aligns tables with the current models.
// The version list and the rules live in configuration/version.
func AutoMigrate(db *gorm.DB) error {
	if err := version.Apply(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(
		&authModel.User{},
		&authModel.RevokedRefreshToken{},
		&friendModel.Friendship{},
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
		&trackingModel.CheckpointRecord{},
		&trackingModel.RedirectResultRecord{},
		&chatModel.Conversation{},
		&chatModel.ConversationUserState{},
		&chatModel.Message{},
		&outbox.Event{},
	); err != nil {
		return err
	}
	return version.ApplyAfterModels(db)
}
