package model

import (
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// Destination is a trip pin. lat/lng always belong to this row.
// locationId is set only after verify or fork-from-Location; then lat/lng copy Location.
type Destination struct {
	utils.Base
	LocationID   *uuid.UUID             `json:"locationId,omitempty" gorm:"type:uuid;index"`
	Location     *Location              `json:"location,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
	Lat          float64                `json:"lat" gorm:"column:lat;not null"`
	Lng          float64                `json:"lng" gorm:"column:lng;not null"`
	Name         string                 `json:"name" gorm:"column:name;type:varchar(255);not null"`
	ArriveTime   *time.Time             `json:"arriveTime,omitempty" gorm:"column:arrive_time"`
	StayOverTime int                    `json:"stayOverTime" gorm:"column:stay_over_time;not null;default:0"`
	Status       enum.DestinationStatus `json:"status" gorm:"column:status;type:varchar(32);not null;default:Editing"`
}

func (Destination) TableName() string {
	return "destinations"
}
