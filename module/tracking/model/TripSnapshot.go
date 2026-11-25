package model

import (
	"time"

	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// TripSnapshot is the current room state sent on join and tracking.resync.
// Observation timestamps let clients distinguish recovered positions from new GPS fixes.
type TripSnapshot struct {
	TripID   string                `json:"tripId"`
	Revision uint64                `json:"revision"`
	Status   enum.TripStatus       `json:"status"`
	Riders   map[string]RiderState `json:"riders"`
}

// Position and redirect fields are omitted for another rider resting privately.
type RiderState struct {
	TrackingStatus        string                `json:"trackingStatus"`
	BranchID              uuid.UUID             `json:"branchId"`
	HopIndex              int                   `json:"hopIndex"`
	Lat                   *float64              `json:"lat,omitempty"`
	Lng                   *float64              `json:"lng,omitempty"`
	Lives                 int                   `json:"lives"`
	ImmuneUntil           time.Time             `json:"immuneUntil"`
	ConnectionStatus      enum.ConnectionStatus `json:"connectionStatus"`
	ObservedAt            time.Time             `json:"observedAt"`
	AwaitingFreshPosition bool                  `json:"awaitingFreshPosition"`
	Redirect              *RedirectCheckpoint   `json:"redirect,omitempty"`
}
