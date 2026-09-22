package model

import (
	"Road-To-Destination-BE/utils"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNilDestination = errors.New("nil destination")
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

func (branch *TripBranch) IsSubBranch() bool {
	// tồn tại 1 trong 2 điểm đầu và kết
	return validDestination(branch.MergeToDestinationID, branch.MergeTo) || validDestination(branch.SplitFromDestinationID, branch.SplitFrom)
}
func validDestination(id *uuid.UUID, des *Destination) bool {
	if id == nil || des == nil {
		return false
	} else if id.String() == des.ID.String() {
		return true
	}
	return false
}
func NewTripBranch(Trip *Trip, splitNode *Destination, mergeNode *Destination) *TripBranch {
	if Trip == nil {
		return nil
	}
	result := &TripBranch{
		Base:   utils.Base{},
		Trip:   Trip,
		TripID: Trip.ID,
	}
	result.Stops = make([]BranchDestination, 0)
	result.SplitFormDestination(splitNode)
	result.MergeToDestination(mergeNode)
	return result
}

// cần arrange lại
func (tripBranch *TripBranch) SplitFormDestination(splitDes *Destination) *TripBranch {
	if splitDes != nil {
		tripBranch.SplitFrom = splitDes
		tripBranch.SplitFromDestinationID = &splitDes.ID
		tripBranch.Stops = append(tripBranch.Stops, *NewBranchDestination(tripBranch, tripBranch.SplitFrom, 0))
	} else {
		tripBranch.SplitFrom = nil
		tripBranch.SplitFromDestinationID = nil
	}
	return tripBranch
}
func (tripBranch *TripBranch) MergeToDestination(mergeDes *Destination) *TripBranch {
	if mergeDes != nil {
		tripBranch.MergeTo = mergeDes
		tripBranch.MergeToDestinationID = &mergeDes.ID
		tripBranch.Stops = append(tripBranch.Stops, *NewBranchDestination(tripBranch, tripBranch.MergeTo, len(tripBranch.Stops)))
	} else {
		tripBranch.MergeTo = nil
		tripBranch.MergeToDestinationID = nil

	}
	return tripBranch
}
