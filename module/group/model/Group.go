package model

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

// Group is a standing club/crew container. One group has many trips later.
type Group struct {
	utils.Base
	Name        string          `json:"name" gorm:"column:name;type:varchar(255);not null"`
	Description string          `json:"description" gorm:"column:description;type:text"`
	OwnerID     uuid.UUID       `json:"ownerId" gorm:"type:uuid;index;not null" swaggertype:"string" format:"uuid"`
	Owner       *authModel.User `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;" swaggerignore:"true"`
	Members     []GroupMember   `json:"members,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (Group) TableName() string {
	return "groups"
}
