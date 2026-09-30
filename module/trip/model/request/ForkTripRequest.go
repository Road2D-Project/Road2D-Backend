package request

import (
	"time"

	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// ForkTripRequest copies a trip. Omitted fields are taken from the source.
// ExcludeUserIDs drops active members of the source. The caller is always the
// leader of the new trip and cannot be excluded. TripType, when set, must
// already have a member-limit policy; the cap is computed from that type and
// is not a free field.
type ForkTripRequest struct {
	Name           *string        `json:"name" binding:"omitempty,min=1,max=255"`
	Note           *string        `json:"note" binding:"omitempty,max=4000"`
	StartTime      *time.Time     `json:"startTime" binding:"omitempty"`
	EndTime        *time.Time     `json:"endTime" binding:"omitempty"`
	TripType       *enum.TripType `json:"tripType" binding:"omitempty" swaggertype:"string" example:"bronze"`
	Visibility     *bool          `json:"visibility" binding:"omitempty"`
	ExcludeUserIDs []uuid.UUID    `json:"excludeUserIds" binding:"omitempty,dive,required" swaggertype:"array,string"`
}
