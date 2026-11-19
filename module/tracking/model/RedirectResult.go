package model

import (
	"time"

	"github.com/google/uuid"
)

type RedirectResult struct {
	JobID         uuid.UUID `json:"jobId"`
	OwnerID       uuid.UUID `json:"ownerId"`
	TripID        uuid.UUID `json:"tripId"`
	MemberID      uuid.UUID `json:"memberId"`
	Generation    uint64    `json:"generation"`
	SchemaVersion int       `json:"schemaVersion"`
	Polyline      string    `json:"polyline,omitempty"`
	DistanceM     float64   `json:"distanceM"`
	DurationS     float64   `json:"durationS"`
	Error         string    `json:"error,omitempty"`
}

// A redelivered job reuses this result instead of calling the provider again.
type RedirectResultRecord struct {
	JobID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	Payload     string    `gorm:"type:jsonb;not null"`
	CompletedAt time.Time `gorm:"not null;index"`
}

func (RedirectResultRecord) TableName() string { return "tracking_redirect_results" }
