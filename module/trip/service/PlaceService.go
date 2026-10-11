package service

import (
	"context"
	"errors"
	"strings"

	mapsclient "Road-To-Destination-BE/module/maps/client"
	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

var (
	// ErrPlaceIDRequired means the caller did not send a Goong place id.
	ErrPlaceIDRequired = errors.New("placeId is required")
	// ErrUnusablePlaceDetail means Goong returned a place that has no name or coordinates.
	ErrUnusablePlaceDetail = errors.New("place detail is missing name or coordinates")
)

// PlaceService các thao tác:
// + model.Destination: địa điểm trong đang được lên plan
// + model.Location: các địa điểm đã được xác thực (get / fork / ensure from place id)
type PlaceService struct {
	destinations DestinationRowRepository
	locations    LocationRowRepository
	places       PlaceDetailClient
}

type DestinationRowRepository interface {
	FindDestinationById(ctx context.Context, destinationId uuid.UUID) (*model.Destination, error)
	UpdateDestination(ctx context.Context, destination *model.Destination) error
	CreateDestination(ctx context.Context, destination *model.Destination) error
}

type LocationRowRepository interface {
	FindLocationById(ctx context.Context, locationId uuid.UUID) (*model.Location, error)
	FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error)
	CreateLocation(ctx context.Context, location *model.Location) error
}

// PlaceDetailClient loads one Goong place. Planning stores that row; it does not edit it later.
type PlaceDetailClient interface {
	DetailPlace(ctx context.Context, req mapRequest.DetailPlaceRequest) (*mapResponse.PlaceDetailResponse, error)
}

func NewLocationService(destinations DestinationRowRepository, locations LocationRowRepository, places PlaceDetailClient) *PlaceService {
	return &PlaceService{destinations: destinations, locations: locations, places: places}
}

func (s *PlaceService) GetLocation(ctx context.Context, locationId uuid.UUID) (*model.Location, error) {
	return s.locations.FindLocationById(ctx, locationId)
}

// EnsureLocation returns the catalog row for a Goong place id.
// A place already stored is returned as-is. A miss is loaded from Goong and inserted.
// created is true only when this call inserted the row.
func (s *PlaceService) EnsureLocation(ctx context.Context, placeID string) (*response.EnsureLocationResponse, bool, error) {
	placeID = strings.TrimSpace(placeID)
	if placeID == "" {
		return nil, false, ErrPlaceIDRequired
	}
	existing, err := s.locations.FindLocationByPlaceID(ctx, placeID)
	if err == nil && existing != nil {
		return ensureLocationResponse(existing), false, nil
	}
	if err != nil && !errors.Is(err, repository.ErrLocationNotFound) {
		return nil, false, err
	}
	if s.places == nil {
		return nil, false, mapsclient.ErrMissingAPIKey
	}
	detail, err := s.places.DetailPlace(ctx, mapRequest.DetailPlaceRequest{PlaceId: placeID})
	if err != nil {
		return nil, false, err
	}
	if detail == nil {
		return nil, false, ErrUnusablePlaceDetail
	}
	row := LocationFromPlaceDetail(*detail)
	if row == nil {
		return nil, false, ErrUnusablePlaceDetail
	}
	if err := s.locations.CreateLocation(ctx, row); err != nil {
		existing, findErr := s.locations.FindLocationByPlaceID(ctx, placeID)
		if findErr == nil && existing != nil {
			return ensureLocationResponse(existing), false, nil
		}
		return nil, false, err
	}
	return ensureLocationResponse(row), true, nil
}

func ensureLocationResponse(location *model.Location) *response.EnsureLocationResponse {
	placeID := ""
	if location.PlaceID != nil {
		placeID = *location.PlaceID
	}
	return &response.EnsureLocationResponse{
		LocationID: location.ID,
		PlaceID:    placeID,
		Name:       location.Name,
		Lat:        location.Lat,
		Lng:        location.Lng,
	}
}

func (s *PlaceService) GetDestination(ctx context.Context, destinationId uuid.UUID) (*model.Destination, error) {
	return s.destinations.FindDestinationById(ctx, destinationId)
}

func (s *PlaceService) UpdateDestination(ctx context.Context, destinationId uuid.UUID, updateRequest request.UpdateDestinationRequest) (*model.Destination, error) {
	targetDes, err := s.destinations.FindDestinationById(ctx, destinationId)
	if err != nil {
		return nil, err
	}
	if targetDes.Status != enum.Editing {
		return nil, repository.ErrDestinationNotEditing
	}
	targetDes.Name = updateRequest.Name
	targetDes.ArriveTime = updateRequest.ArriveTime
	targetDes.Status = updateRequest.Status
	targetDes.StayOverTime = updateRequest.StayOverTime
	if targetDes.CoordinatesEditable() {
		targetDes.Lat = updateRequest.Lat
		targetDes.Lng = updateRequest.Lng
	}
	if err := s.destinations.UpdateDestination(ctx, targetDes); err != nil {
		return nil, err
	}
	return targetDes, nil
}

func (s *PlaceService) ForkLocation(ctx context.Context, locationId uuid.UUID, forkRequest request.ForkLocationRequest) (*response.CreateDestinationResponse, error) {
	targetLocation, err := s.locations.FindLocationById(ctx, locationId)
	if err != nil {
		return nil, err
	}
	destination := ForkLocationToDestination(targetLocation)
	if forkRequest.Name != "" {
		destination.Name = forkRequest.Name
	}
	destination.StayOverTime = forkRequest.StayOverTime
	destination.ArriveTime = forkRequest.ArriveTime
	if err := s.destinations.CreateDestination(ctx, destination); err != nil {
		return nil, err
	}
	return &response.CreateDestinationResponse{
		DestinationId: destination.ID,
		LocationId:    destination.LocationID,
		PlaceId:       targetLocation.PlaceID,
	}, nil
}

func ForkLocationToDestination(location *model.Location) *model.Destination {
	if location == nil {
		return nil
	}
	return &model.Destination{
		Base:         utils.Base{},
		LocationID:   &location.ID,
		Location:     location,
		Lat:          location.Lat,
		Lng:          location.Lng,
		Name:         location.Name,
		ArriveTime:   nil,
		StayOverTime: 0,
		Status:       enum.Editing,
	}
}
