package service

import (
	"Road-To-Destination-BE/utils/enum"
	"context"
	"time"

	"Road-To-Destination-BE/module/maps/model"
	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

type DirectionMapClient interface {
	Direction(ctx context.Context, req request.DirectionRequest) (*response.DirectionResponse, error)
}

type LocationLegStore interface {
	Find(ctx context.Context, origin, destination string, vehicle enum.Vehicle) (*model.LocationLeg, error)
	Upsert(ctx context.Context, leg *model.LocationLeg) error
}

type DirectionService struct {
	mapClient DirectionMapClient
	legs      LocationLegStore
}

func NewDirectionService(mapClient DirectionMapClient, legs LocationLegStore) *DirectionService {
	return &DirectionService{mapClient: mapClient, legs: legs}
}

func (srv *DirectionService) Route(ctx context.Context, req request.DirectionRequest) (*model.LocationLeg, error) {
	vehicle, err := req.Vehicle.Normalized()
	if err != nil {
		return nil, err
	}
	req.Vehicle = vehicle

	if !req.Alternatives {
		cached, err := srv.legs.Find(ctx, req.Origin, req.Destination, req.Vehicle)
		if err != nil {
			return nil, err
		}
		if cached != nil {
			return cached, nil
		}
	}

	raw, err := srv.mapClient.Direction(ctx, req)
	if err != nil {
		return nil, err
	}

	leg := locationLegFromRoute(req, raw.Routes[0])
	if err := srv.legs.Upsert(ctx, leg); err != nil {
		return nil, err
	}
	return leg, nil
}

func locationLegFromRoute(req request.DirectionRequest, route response.Route) *model.LocationLeg {
	distanceM := 0
	durationS := 0
	steps := make([]model.RouteStep, 0)
	for _, leg := range route.Legs {
		distanceM += leg.Distance.Value
		durationS += leg.Duration.Value
		for _, step := range leg.Steps {
			steps = append(steps, model.RouteStep{
				Instruction: step.HTMLInstructions,
				Maneuver:    step.Maneuver,
				DistanceM:   step.Distance.Value,
				DurationS:   step.Duration.Value,
				Polyline:    step.Polyline.Points,
				Start: model.LatLng{
					Lat: step.StartLocation.Lat,
					Lng: step.StartLocation.Lng,
				},
				End: model.LatLng{
					Lat: step.EndLocation.Lat,
					Lng: step.EndLocation.Lng,
				},
				TravelMode: step.TravelMode,
			})
		}
	}
	return &model.LocationLeg{
		Origin:      req.Origin,
		Destination: req.Destination,
		Vehicle:     req.Vehicle,
		DistanceM:   distanceM,
		DurationS:   durationS,
		Polyline:    route.OverviewPolyline.Points,
		Steps:       steps,
		ComputedAt:  time.Now().UTC(),
	}
}
