package model

import (
	"time"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// TripMember is a user's seat on one trip. Role and Status omit gorm "default"
// tags: GORM treats int-backed enumer zero values (leader/invited) as empty.
type TripMember struct {
	utils.Base
	TripID   uuid.UUID             `json:"tripId" gorm:"type:uuid;uniqueIndex:idx_trip_user;index;not null" swaggertype:"string" format:"uuid"`
	Trip     *Trip                 `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	UserID   uuid.UUID             `json:"userId" gorm:"type:uuid;uniqueIndex:idx_trip_user;index;not null" swaggertype:"string" format:"uuid"`
	User     *authModel.User       `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	Role     enum.TripRole         `json:"role" gorm:"column:role;type:varchar(16);not null" swaggertype:"string" example:"member"`
	Status   enum.MembershipStatus `json:"status" gorm:"column:status;type:varchar(16);not null;index" swaggertype:"string" example:"active"`
	Nickname string                `json:"nickname" gorm:"column:nickname;type:varchar(64);not null"`
	JoinedAt *time.Time            `json:"joinedAt,omitempty" gorm:"column:joined_at"`
}

func (TripMember) TableName() string {
	return "trip_members"
}
