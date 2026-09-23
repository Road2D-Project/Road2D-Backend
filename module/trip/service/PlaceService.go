package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

// PlaceService các thao tác:
// + model.Destination: địa điểm trong đang được lên plan
// + model.Location: các địa điểm đã được xác thực (get / fork)
type PlaceService struct {
	destinations DestinationRowRepository
	locations    LocationRowRepository
}

type DestinationRowRepository interface {
	FindDestinationById(ctx context.Context, destinationId uuid.UUID) (*model.Destination, error)
	UpdateDestination(ctx context.Context, destination *model.Destination) error
	CreateDestination(ctx context.Context, destination *model.Destination) error
}

type LocationRowRepository interface {
	FindLocationById(ctx context.Context, locationId uuid.UUID) (*model.Location, error)
}

func NewLocationService(destinations DestinationRowRepository, locations LocationRowRepository) *PlaceService {
	return &PlaceService{destinations: destinations, locations: locations}
}

func (s *PlaceService) GetLocation(ctx context.Context, locationId uuid.UUID) (*model.Location, error) {
	return s.locations.FindLocationById(ctx, locationId)
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
