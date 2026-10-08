package model

import (
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
}
