package model

import (
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// Travel is a frozen snapshot of a Leg between two Destinations when the trip locks.
// The unique key carries TripID: two trips may walk the same destination pair, and
// one trip freezing its row must not lock the other one out.
type Travel struct {
	utils.Base
	TripID            uuid.UUID    `json:"tripId" gorm:"type:uuid;uniqueIndex:idx_travel_pair;index;not null"`
	Trip              *Trip        `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	FromDestinationID uuid.UUID    `json:"fromDestinationId" gorm:"type:uuid;uniqueIndex:idx_travel_pair;index;not null"`
	FromDestination   *Destination `json:"fromDestination,omitempty" gorm:"foreignKey:FromDestinationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	ToDestinationID   uuid.UUID    `json:"toDestinationId" gorm:"type:uuid;uniqueIndex:idx_travel_pair;index;not null"`
	ToDestination     *Destination `json:"toDestination,omitempty" gorm:"foreignKey:ToDestinationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	LegID             *uuid.UUID   `json:"legId,omitempty" gorm:"type:uuid;index"`
	Leg               *Leg         `json:"leg,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Vehicle           enum.Vehicle `json:"vehicle" gorm:"column:vehicle;type:varchar(16);uniqueIndex:idx_travel_pair;not null"`
	Polyline          string       `json:"polyline" gorm:"column:polyline;type:text"`
	DistanceM         float64      `json:"distanceM" gorm:"column:distance_m;not null;default:0"`
	DurationS         float64      `json:"durationS" gorm:"column:duration_s;not null;default:0"`
	IsFrozen          bool         `json:"isFrozen" gorm:"column:is_frozen;not null;default:false"`
	LastComputedAt    time.Time    `json:"lastComputedAt" gorm:"column:last_computed_at"`
}

func (Travel) TableName() string {
	return "travels"
}

// Expired reports whether the snapshot is too old to reuse. A frozen travel is
// history and never expires.
func (travel *Travel) Expired(now time.Time, fallback time.Duration) bool {
	if travel == nil {
		return true
	}
	if travel.IsFrozen {
		return false
	}
	return travel.LastComputedAt.Add(fallback).Before(now)
}
