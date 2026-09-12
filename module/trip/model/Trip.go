package model

import (
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"time"
)

// Trip is the product itinerary (not the Goong TSP DTO in module/maps).
type Trip struct {
	utils.Base
	Name          string          `json:"name" gorm:"column:name;type:varchar(255);not null"`
	Status        enum.TripStatus `json:"status" gorm:"column:status;type:varchar(32);not null;default:planning"`
	StartTime     *time.Time      `json:"startTime,omitempty" gorm:"column:start_time"`
	EndTime       *time.Time      `json:"endTime,omitempty" gorm:"column:end_time"`
	TotalDistance float64         `json:"totalDistance" gorm:"column:total_distance;not null;default:0"`
	Note          string          `json:"note" gorm:"column:note;type:text"`
	Branches      []TripBranch    `json:"branches,omitempty" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}

func (Trip) TableName() string {
	return "trips"
}
