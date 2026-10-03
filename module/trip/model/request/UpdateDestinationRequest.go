package request

import (
	"Road-To-Destination-BE/utils/enum"
	"time"
)

type UpdateDestinationRequest struct {
	Lng          float64                `json:"lng"`
	Lat          float64                `json:"lat" `
	Name         string                 `json:"name"`
	ArriveTime   *time.Time             `json:"arriveTime,omitempty" binding:"omitempty,arriveTime"`
	StayOverTime int                    `json:"stayOverTime" binding:"stayMinutes"`
	Status       enum.DestinationStatus `json:"status"`
}
