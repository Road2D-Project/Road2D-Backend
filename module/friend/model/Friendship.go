package model

import (
	"bytes"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// Friendship is one row for an unordered pair of users.
// UserLowID is the smaller UUID so A→B and B→A cannot both exist.
type Friendship struct {
	utils.Base
	UserLowID   uuid.UUID         `json:"userLowId" gorm:"type:uuid;uniqueIndex:idx_friend_pair;not null" swaggertype:"string" format:"uuid"`
	UserLow     *authModel.User   `json:"-" gorm:"foreignKey:UserLowID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	UserHighID  uuid.UUID         `json:"userHighId" gorm:"type:uuid;uniqueIndex:idx_friend_pair;not null" swaggertype:"string" format:"uuid"`
	UserHigh    *authModel.User   `json:"-" gorm:"foreignKey:UserHighID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" swaggerignore:"true"`
	RequesterID uuid.UUID         `json:"requesterId" gorm:"type:uuid;not null" swaggertype:"string" format:"uuid"`
	Status      enum.FriendStatus `json:"status" gorm:"column:status;type:varchar(16);not null;index" swaggertype:"string" example:"pending"`
}

func (Friendship) TableName() string {
	return "friendships"
}

// OrderedPair returns the smaller UUID first.
func OrderedPair(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if bytes.Compare(a[:], b[:]) <= 0 {
		return a, b
	}
	return b, a
}

// OtherUser is the id in the pair that is not userID.
func (row Friendship) OtherUser(userID uuid.UUID) uuid.UUID {
	if row.UserLowID == userID {
		return row.UserHighID
	}
	return row.UserLowID
}
