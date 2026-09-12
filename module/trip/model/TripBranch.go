package model

import (
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

type TripBranch struct {
	utils.Base
	TripID                 uuid.UUID           `json:"tripId" gorm:"type:uuid;index;not null"`
	Trip                   *Trip               `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	SplitFromDestinationID *uuid.UUID          `json:"splitFromDestinationId,omitempty" gorm:"type:uuid;index"`
	SplitFrom              *Destination        `json:"splitFrom,omitempty" gorm:"foreignKey:SplitFromDestinationID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	MergeToDestinationID   *uuid.UUID          `json:"mergeToDestinationId,omitempty" gorm:"type:uuid;index"`
	MergeTo                *Destination        `json:"mergeTo,omitempty" gorm:"foreignKey:MergeToDestinationID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Label                  string              `json:"label" gorm:"column:label;type:varchar(255)"`
	Stops                  []BranchDestination `json:"stops,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (TripBranch) TableName() string {
	return "trip_branches"
}
