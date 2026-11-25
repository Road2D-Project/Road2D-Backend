package model

import (
	"time"

	"github.com/google/uuid"
)

const RedirectSchemaVersion = 1

type RedirectJob struct {
	JobID         uuid.UUID `json:"jobId"`
	OwnerID       uuid.UUID `json:"ownerId"`
	TripID        uuid.UUID `json:"tripId"`
	MemberID      uuid.UUID `json:"memberId"`
	Generation    uint64    `json:"generation"`
	SchemaVersion int       `json:"schemaVersion"`
	Origin        Point     `json:"origin"`
	Target        Point     `json:"target"`
	ExpiresAt     time.Time `json:"expiresAt"`
}
