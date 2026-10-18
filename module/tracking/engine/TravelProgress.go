package engine

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
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
	BatteryPercent   int
	SignalBars       int
	ObservedAt       time.Time

	// Deviation counters. OffRouteSamples counts consecutive samples farther
	// than geo.OffRouteM from the branch polyline. OnRouteSamples counts
	// consecutive samples back on it. Only one of them is non-zero.
	OffRouteSamples int
	OnRouteSamples  int
	ArriveSamples   int
	Lives           int
	ImmuneUntil     time.Time
	Arrived         map[uuid.UUID]struct{}

	// ResumeDrive is the drive status to restore when stay mode ends.
	// HasResume reports that a value was saved.
	ResumeDrive enum.DriveStatus
	HasResume   bool

	// Latest snap facts copied from the worker. LastSnapAt is the sample the
	// worker measured; LastCountedAt is the sample the room already counted.
	LastSnapAt    time.Time
	LastCountedAt time.Time
	LastDistM     float64
	OffRoute      bool

	// Redirect is the temporary route toward the next stop while deviated.
	// nil when the rider is on the branch polyline.
	Redirect           *RedirectState
	RedirectGeneration uint64
	RedirectSeg        int
	RedirectProgress   float64
	RedirectDistM      float64
	RedirectArrived    bool
	redirects          chan RedirectResult

	// jobs, updates, and cancel belong to the room goroutine.
	// The snap loop only receives jobs and sends SnapUpdate.
	jobs         chan snapJob
	updates      chan SnapUpdate
	cancel       context.CancelFunc
	LastSegIndex int
	Progress     float64
	Route        *geo.Route
}

// RedirectState is one temporary route. Generation ties worker results and
// snap updates to this attempt; a result from an older attempt is dropped.
type RedirectState struct {
	Generation  uint64
	Reason      string // why this attempt was made: confirmed or refresh
	Retries     int    // provider retries already spent on this origin
	Hop         int    // hop index the rider was on when the route was requested
	Origin      model.Point
	Target      model.Stop
	Polyline    string
	Route       *geo.Route // nil until the router answers
	DistanceM   float64
	DurationS   float64
	RequestedAt time.Time
	RetryAt     time.Time // zero means no retry is scheduled
	Pending     bool
	Failed      bool
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
		OnRouteSamples:   0,
		ArriveSamples:    0,
		Lives:            LivesStart,
		ImmuneUntil:      time.Time{},
		Arrived:          nil,
		ConnectionStatus: enum.CONNECTING,
		LastSegIndex:     0,
		Progress:         0,
		Route:            nil,
		redirects:        make(chan RedirectResult, redirectResultBuffer),
	}
}
