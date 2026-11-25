package model

import (
	"time"

	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// TripCheckpoint contains values owned by the room. It never contains channels,
// cancellation functions, connections, or geometry objects with mutable cursors.
type TripCheckpoint struct {
	TripID     uuid.UUID                     `json:"tripId"`
	Sequence   uint64                        `json:"sequence"`
	Revision   uint64                        `json:"revision"`
	Status     enum.TripStatus               `json:"status"`
	CapturedAt time.Time                     `json:"capturedAt"`
	Riders     map[uuid.UUID]RiderCheckpoint `json:"riders"`
}

type RiderCheckpoint struct {
	BranchID           uuid.UUID           `json:"branchId"`
	HopIndex           int                 `json:"hopIndex"`
	LastSegIndex       int                 `json:"lastSegIndex"`
	Progress           float64             `json:"progress"`
	DriveStatus        enum.DriveStatus    `json:"driveStatus"`
	Lat                float64             `json:"lat"`
	Lng                float64             `json:"lng"`
	BatteryPercent     int                 `json:"batteryPercent"`
	SignalBars         int                 `json:"signalBars"`
	ObservedAt         time.Time           `json:"observedAt"`
	Lives              int                 `json:"lives"`
	ImmuneUntil        time.Time           `json:"immuneUntil"`
	Arrived            []uuid.UUID         `json:"arrived,omitempty"`
	ResumeDrive        enum.DriveStatus    `json:"resumeDrive"`
	HasResume          bool                `json:"hasResume"`
	RedirectGeneration uint64              `json:"redirectGeneration"`
	RedirectSeg        int                 `json:"redirectSeg"`
	RedirectProgress   float64             `json:"redirectProgress"`
	Redirect           *RedirectCheckpoint `json:"redirect,omitempty"`
}

type RedirectCheckpoint struct {
	Generation  uint64    `json:"generation"`
	Reason      string    `json:"reason"`
	Retries     int       `json:"retries"`
	Hop         int       `json:"hop"`
	Origin      Point     `json:"origin"`
	Target      Stop      `json:"target"`
	Polyline    string    `json:"polyline"`
	DistanceM   float64   `json:"distanceM"`
	DurationS   float64   `json:"durationS"`
	RequestedAt time.Time `json:"requestedAt"`
	RetryAt     time.Time `json:"retryAt"`
	Pending     bool      `json:"pending"`
	Failed      bool      `json:"failed"`
}

// CheckpointRecord is the durable envelope used by conditional PostgreSQL upserts.
type CheckpointRecord struct {
	TripID   uuid.UUID `gorm:"type:uuid;primaryKey"`
	Sequence uint64    `gorm:"not null"`
	Payload  string    `gorm:"type:jsonb;not null"`
	SavedAt  time.Time `gorm:"not null"`
}

func (CheckpointRecord) TableName() string { return "tracking_checkpoints" }
