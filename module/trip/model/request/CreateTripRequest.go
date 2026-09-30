package request

import (
	"time"

	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// CreateTripRequest opens a planning trip. The caller becomes leader.
// MemberUserIDs are seated immediately as members; there is no friend graph
// yet, so any account id is accepted. TripType freezes MemberLimit.
// Visibility true publishes the trip for discovery.
// MainBranch, when set, is the response of POST /planning/preview. That
// reviewed route becomes the trip's initial main branch.
type CreateTripRequest struct {
	Name          string                             `json:"name" binding:"required,min=1,max=255"`
	Note          string                             `json:"note" binding:"max=4000"`
	StartTime     *time.Time                         `json:"startTime" binding:"omitempty"`
	EndTime       *time.Time                         `json:"endTime" binding:"omitempty"`
	TripType      *enum.TripType                     `json:"tripType" binding:"required" swaggertype:"string" example:"bronze"`
	Visibility    bool                               `json:"visibility"`
	MemberUserIDs []uuid.UUID                        `json:"memberUserIds" binding:"required,min=1,dive,required" swaggertype:"array,string"`
	MainBranch    *response.PreviewLocationsResponse `json:"mainBranch,omitempty"`
}
