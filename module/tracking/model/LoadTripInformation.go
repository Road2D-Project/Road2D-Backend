package model

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrTripNotActive = errors.New("Trip is not active")
)

// LoadTripInformation reads the trip once for a tracking room.
// Callers treat the result as read-only after it returns.
func LoadTripInformation(db *gorm.DB, roomID string) (*TripInformation, error) {
	tripID, err := uuid.Parse(roomID)
	if err != nil {
		return nil, err
	}
	_, err = loadActiveTrip(db, roomID)
	if err != nil {
		return nil, err
	}
	out := &TripInformation{
		TripId:   tripID,
		Riders:   map[uuid.UUID]*Rider{},
		Branches: map[uuid.UUID]*TravelBranch{},
		Status:   enum.TripActive,
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
func loadActiveTrip(db *gorm.DB, roomID string) (*model.Trip, error) {
	var out model.Trip
	err := db.Table(out.TableName()).Where("id = ?", roomID).First(&out).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}
	if out.Status != enum.TripActive {
		return nil, ErrTripNotActive
	}
	return &out, nil
}
