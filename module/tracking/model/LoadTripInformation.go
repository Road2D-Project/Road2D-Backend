package model

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LoadTripInformation reads the trip once for a tracking room.
// Callers treat the result as read-only after it returns.
func LoadTripInformation(db *gorm.DB, roomID string) (*TripInformation, error) {
	tripID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, err
	}
	out := &TripInformation{
		TripId:   tripID,
		Riders:   map[uuid.UUID]*Rider{},
		Branches: map[uuid.UUID]*TravelBranch{},
	}
	if err := LoadTripMeta(db, out); err != nil {
		return nil, err
	}
	if err := LoadRiders(db, out); err != nil {
		return nil, err
	}
	if err := LoadBranches(db, out); err != nil {
		return nil, err
	}
	travels, err := LoadTravels(db, tripID)
	if err != nil {
		return nil, err
	}
	for _, branch := range out.Branches {
		assembleHops(branch, travels)
		if branch.SplitFrom == nil && branch.MergeTo == nil && out.MainBranchID == uuid.Nil {
			out.MainBranchID = branch.ID
		}
	}
	for _, rider := range out.Riders {
		if _, ok := out.Branches[rider.AssignedBranchID]; !ok {
			rider.AssignedBranchID = uuid.Nil
		}
	}
	return out, nil
}
