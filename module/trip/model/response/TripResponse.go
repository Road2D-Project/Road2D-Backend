package response

import (
	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"context"
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
	ImageURL      *string         `json:"imageUrl,omitempty"`
	ImagePath     *string         `json:"-" swaggerignore:"true"`
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
		ImagePath:     trip.ImagePath,
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

type ImageURLSigner interface {
	DownloadURL(context.Context, string, string) (string, error)
}

func (r *TripResponse) ResolveImageURLs(ctx context.Context, signer ImageURLSigner) error {
	r.ImageURL = nil
	if r.ImagePath == nil || *r.ImagePath == "" {
		return nil
	}
	if err := objectStorage.ValidateOwnedPath(r.ID.String(), *r.ImagePath); err != nil {
		return err
	}
	url, err := signer.DownloadURL(ctx, objectStorage.TripNamespace, *r.ImagePath)
	if err != nil {
		return err
	}
	r.ImageURL = &url
	return nil
}

func (r *TripListResponse) ResolveImageURLs(ctx context.Context, signer ImageURLSigner) error {
	for i := range r.Trips {
		if err := r.Trips[i].ResolveImageURLs(ctx, signer); err != nil {
			return err
		}
	}
	return nil
}
