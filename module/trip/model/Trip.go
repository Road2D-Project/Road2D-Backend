package model

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	groupmodel "Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// Trip is the product itinerary (not the Goong TSP DTO in module/maps).
type Trip struct {
	utils.Base
	GroupID       *uuid.UUID        `json:"groupId,omitempty" gorm:"type:uuid;index" swaggertype:"string" format:"uuid"`
	Group         *groupmodel.Group `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" swaggerignore:"true"`
	OwnerID       uuid.UUID         `json:"ownerId" gorm:"type:uuid;index;not null" swaggertype:"string" format:"uuid"`
	Owner         *authModel.User   `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" swaggerignore:"true"`
	Name          string            `json:"name" gorm:"column:name;type:varchar(255);not null"`
	Status        enum.TripStatus   `json:"status" gorm:"column:status;type:varchar(32);not null"`
	StartTime     *time.Time        `json:"startTime,omitempty" gorm:"column:start_time"`
	EndTime       *time.Time        `json:"endTime,omitempty" gorm:"column:end_time"`
	TotalDistance float64           `json:"totalDistance" gorm:"column:total_distance;not null;default:0"`
	Note          string            `json:"note" gorm:"column:note;type:text"`
	InviteToken   string            `json:"-" gorm:"column:invite_token;type:varchar(64);uniqueIndex;not null" swaggerignore:"true"`
	Branches      []TripBranch      `json:"branches,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Members       []TripMember      `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
}

func (Trip) TableName() string {
	return "trips"
}
