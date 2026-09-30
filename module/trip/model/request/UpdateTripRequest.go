package request

import "time"

// UpdateTripRequest patches name, note, times, and visibility.
// TripType and memberLimit are absent on purpose: both are frozen at creation
// because they belong to the subscription policy.
type UpdateTripRequest struct {
	Name       *string    `json:"name" binding:"omitempty,min=1,max=255"`
	Note       *string    `json:"note" binding:"omitempty,max=4000"`
	StartTime  *time.Time `json:"startTime" binding:"omitempty"`
	EndTime    *time.Time `json:"endTime" binding:"omitempty"`
	Visibility *bool      `json:"visibility" binding:"omitempty"`
}
