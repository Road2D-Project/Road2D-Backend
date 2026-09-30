package response

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

// TripResponse is the non-sensitive trip payload. The invite token is never
// included; mint it with the invite-link endpoint.
type TripResponse struct {
	ID            uuid.UUID       `json:"id" swaggertype:"string" format:"uuid"`
	OwnerID       uuid.UUID       `json:"ownerId" swaggertype:"string" format:"uuid"`
	Name          string          `json:"name"`
	Status        enum.TripStatus `json:"status" swaggertype:"string" example:"planning"`
	TripType      enum.TripType   `json:"tripType" swaggertype:"string" example:"bronze"`
	MemberLimit   int             `json:"memberLimit" example:"15"`
	Visibility    bool            `json:"visibility"`
	StartTime     *time.Time      `json:"startTime,omitempty"`
	EndTime       *time.Time      `json:"endTime,omitempty"`
	TotalDistance float64         `json:"totalDistance"`
	Note          string          `json:"note"`
	MyRole        *enum.TripRole  `json:"myRole,omitempty" swaggertype:"string" example:"leader"`
	CreatedTime   time.Time       `json:"createdTime"`
	UpdatedTime   time.Time       `json:"updatedTime"`
}

type TripListResponse struct {
	Trips []TripResponse `json:"trips"`
}

func FromTrip(trip *model.Trip, role *enum.TripRole) TripResponse {
	return TripResponse{
		ID:            trip.ID,
		OwnerID:       trip.OwnerID,
		Name:          trip.Name,
		Status:        trip.Status,
		TripType:      trip.TripType,
		MemberLimit:   trip.MemberLimit,
		Visibility:    trip.Visibility,
		StartTime:     trip.StartTime,
		EndTime:       trip.EndTime,
		TotalDistance: trip.TotalDistance,
		Note:          trip.Note,
		MyRole:        role,
		CreatedTime:   trip.CreatedAt,
		UpdatedTime:   trip.UpdatedAt,
	}
}

func FromTripPtr(trip *model.Trip, role *enum.TripRole) *TripResponse {
	out := FromTrip(trip, role)
	return &out
}

func RolePtr(role enum.TripRole) *enum.TripRole {
	copied := role
	return &copied
}
