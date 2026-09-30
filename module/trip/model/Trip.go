package model

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// Trip is the product itinerary (not the Goong TSP DTO in module/maps).
// It does not belong to a standing group. MemberLimit and TripType are set
// once, from the tier, and are not patched later.
// Visibility true means any authenticated user may find the trip; false means
// only an active member can read it.
type Trip struct {
	utils.Base
	OwnerID       uuid.UUID       `json:"ownerId" gorm:"type:uuid;index;not null" swaggertype:"string" format:"uuid"`
	Owner         *authModel.User `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" swaggerignore:"true"`
	Name          string          `json:"name" gorm:"column:name;type:varchar(255);not null"`
	Status        enum.TripStatus `json:"status" gorm:"column:status;type:varchar(32);not null"`
	TripType      enum.TripType   `json:"tripType" gorm:"column:trip_type;type:varchar(16);not null" swaggertype:"string" example:"bronze"`
	MemberLimit   int             `json:"memberLimit" gorm:"column:member_limit;not null"`
	Visibility    bool            `json:"visibility" gorm:"column:visibility;not null;index"`
	StartTime     *time.Time      `json:"startTime,omitempty" gorm:"column:start_time"`
	EndTime       *time.Time      `json:"endTime,omitempty" gorm:"column:end_time"`
	TotalDistance float64         `json:"totalDistance" gorm:"column:total_distance;not null;default:0"`
	Note          string          `json:"note" gorm:"column:note;type:text"`
	InviteToken   string          `json:"-" gorm:"column:invite_token;type:varchar(64);uniqueIndex;not null" swaggerignore:"true"`
	Branches      []TripBranch    `json:"branches,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Members       []TripMember    `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
}

func (Trip) TableName() string {
	return "trips"
}
