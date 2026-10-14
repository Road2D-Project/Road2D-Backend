package engine

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// Run time data
type TravelProgress struct {
	UserID           uuid.UUID
	BranchID         uuid.UUID
	HopIndex         int
	DriveStatus      enum.DriveStatus
	ConnectionStatus enum.ConnectionStatus
	Lat              float64
	Lng              float64
	ObservedAt       time.Time
	OffRouteSamples  int
	ArriveSamples    int
	Lives            int
	ImmuneUntil      time.Time
	Arrived          map[uuid.UUID]struct{}

	// jobs, updates, and cancel belong to the room goroutine.
	// The snap loop only receives jobs and sends SnapUpdate.
	jobs             chan snapJob
	updates          chan SnapUpdate
	cancel           context.CancelFunc
	LastSegIndex     int
	Progress         float64
	Route            *geo.Route
	CompletedChannel *chan uuid.UUID
	OnRiderComplete  OnComplete
}
type OnComplete func(prg *TravelProgress) error

func NewTravelProgress(riderID uuid.UUID, branch uuid.UUID) *TravelProgress {
	return &TravelProgress{
		UserID:           riderID,
		BranchID:         branch,
		HopIndex:         0,
		DriveStatus:      enum.OnROUTE,
		Lat:              0,
		Lng:              0,
		ObservedAt:       time.Time{},
		OffRouteSamples:  0,
		ArriveSamples:    0,
		Lives:            0,
		ImmuneUntil:      time.Time{},
		Arrived:          nil,
		ConnectionStatus: enum.CONNECTING,
		LastSegIndex:     0,
		Progress:         0,
		Route:            nil,
	}
}
