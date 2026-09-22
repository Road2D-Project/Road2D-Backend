package model

import (
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

type BranchDestination struct {
	utils.Base
	TripBranchID  uuid.UUID    `json:"tripBranchId" gorm:"type:uuid;uniqueIndex:idx_branch_dest;uniqueIndex:idx_branch_order;not null"`
	TripBranch    *TripBranch  `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	DestinationID uuid.UUID    `json:"destinationId" gorm:"type:uuid;uniqueIndex:idx_branch_dest;index;not null"`
	Destination   *Destination `json:"destination,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	OrderInBranch int          `json:"orderInBranch" gorm:"column:order_in_branch;uniqueIndex:idx_branch_order;not null"`
}

func (BranchDestination) TableName() string {
	return "branch_destinations"
}

func NewBranchDestination(TripBranch *TripBranch, Destination *Destination, orderInt int) *BranchDestination {
	return &BranchDestination{
		TripBranchID:  TripBranch.ID,
		TripBranch:    TripBranch,
		DestinationID: Destination.ID,
		Destination:   Destination,
		OrderInBranch: orderInt,
	}
}
