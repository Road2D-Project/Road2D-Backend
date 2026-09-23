package request

import (
	"Road-To-Destination-BE/utils/enum"
	"time"
)

type UpdateDestinationRequest struct {
	Lng          float64                `json:"lng"`
	Lat          float64                `json:"lat" `
	Name         string                 `json:"name"`
	ArriveTime   *time.Time             `json:"arriveTime,omitempty"`
	StayOverTime int                    `json:"stayOverTime"`
	Status       enum.DestinationStatus `json:"status"`
}
