package model

import (
	"encoding/json"
	"time"

	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// Leg is the shared A→B route cache, keyed by coordinates + vehicle.
// Location FKs are optional (pin-to-pin compute does not need a verified place).
type Leg struct {
	utils.Base
	FromLat        float64         `json:"fromLat" gorm:"column:from_lat;uniqueIndex:idx_leg_cache;not null"`
	FromLng        float64         `json:"fromLng" gorm:"column:from_lng;uniqueIndex:idx_leg_cache;not null"`
	ToLat          float64         `json:"toLat" gorm:"column:to_lat;uniqueIndex:idx_leg_cache;not null"`
	ToLng          float64         `json:"toLng" gorm:"column:to_lng;uniqueIndex:idx_leg_cache;not null"`
	FromLocationID *uuid.UUID      `json:"fromLocationId,omitempty" gorm:"type:uuid;index"`
	FromLocation   *Location       `json:"fromLocation,omitempty" gorm:"foreignKey:FromLocationID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	ToLocationID   *uuid.UUID      `json:"toLocationId,omitempty" gorm:"type:uuid;index"`
	ToLocation     *Location       `json:"toLocation,omitempty" gorm:"foreignKey:ToLocationID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Vehicle        enum.Vehicle    `json:"vehicle" gorm:"column:vehicle;type:varchar(16);uniqueIndex:idx_leg_cache;not null"`
	Polyline       string          `json:"polyline" gorm:"column:polyline;type:text"`
	DistanceM      float64         `json:"distanceM" gorm:"column:distance_m;not null;default:0"`
	DurationS      float64         `json:"durationS" gorm:"column:duration_s;not null;default:0"`
	Steps          json.RawMessage `json:"steps,omitempty" gorm:"column:steps;type:jsonb;serializer:json"`
	Source         enum.LegSource  `json:"source" gorm:"column:source;type:varchar(32);not null;default:direction"`
	LastComputedAt time.Time       `json:"lastComputedAt" gorm:"column:last_computed_at"`
	TTLSeconds     int             `json:"ttlSeconds" gorm:"column:ttl_seconds;not null;default:0"`
}

func (Leg) TableName() string {
	return "legs"
}

func (leg *Leg) SetSteps(steps []RouteStep) error {
	if len(steps) == 0 {
		leg.Steps = nil
		return nil
	}
	raw, err := json.Marshal(steps)
	if err != nil {
		return err
	}
	leg.Steps = raw
	return nil
}

func (leg *Leg) StepList() ([]RouteStep, error) {
	if len(leg.Steps) == 0 {
		return []RouteStep{}, nil
	}
	var steps []RouteStep
	if err := json.Unmarshal(leg.Steps, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}
