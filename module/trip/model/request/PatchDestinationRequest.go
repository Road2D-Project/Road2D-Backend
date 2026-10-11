package request

import (
	"time"

	"Road-To-Destination-BE/utils/enum"
)

// PatchDestinationRequest updates only the fields that are sent.
// lat/lng are applied only when the pin is not linked to a location.
type PatchDestinationRequest struct {
	Name         *string                 `json:"name" binding:"omitempty,min=1,max=255"`
	ArriveTime   *time.Time              `json:"arriveTime,omitempty" binding:"omitempty,arriveTime"`
	StayOverTime *int                    `json:"stayOverTime" binding:"omitempty,stayMinutes"`
	Status       *enum.DestinationStatus `json:"status,omitempty" swaggertype:"string" example:"editing"`
	Lat          *float64                `json:"lat,omitempty"`
	Lng          *float64                `json:"lng,omitempty"`
}
