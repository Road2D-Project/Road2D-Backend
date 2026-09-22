package response

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

type TripResponse struct {
	ID            uuid.UUID       `json:"id" swaggertype:"string" format:"uuid"`
	GroupID       *uuid.UUID      `json:"groupId,omitempty" swaggertype:"string" format:"uuid"`
	OwnerID       uuid.UUID       `json:"ownerId" swaggertype:"string" format:"uuid"`
	Name          string          `json:"name"`
	Status        enum.TripStatus `json:"status" swaggertype:"string" example:"planning"`
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
		GroupID:       trip.GroupID,
		OwnerID:       trip.OwnerID,
		Name:          trip.Name,
		Status:        trip.Status,
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
