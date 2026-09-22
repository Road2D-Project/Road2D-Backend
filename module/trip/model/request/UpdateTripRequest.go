package request

import "time"

type UpdateTripRequest struct {
	Name      *string    `json:"name" binding:"omitempty,min=1,max=255"`
	Note      *string    `json:"note" binding:"omitempty,max=4000"`
	StartTime *time.Time `json:"startTime" binding:"omitempty"`
	EndTime   *time.Time `json:"endTime" binding:"omitempty"`
}
