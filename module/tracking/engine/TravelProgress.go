package engine

import (
	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

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

	PendingPoint chan model.Point
	LastSegIndex int     // số thứ tự segment hiện tại
	Progress     float64 // số m đi được
	Route        *geo.Route
}

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
		PendingPoint:     make(chan model.Point, 1),
		LastSegIndex:     0,
		Progress:         0,
		Route:            &geo.Route{},
	}
}
