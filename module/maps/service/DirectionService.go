package service

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
)

type DirectionMapClient interface {
	Direction(ctx context.Context, req request.DirectionRequest) (*response.DirectionResponse, error)
}

type DirectionService struct {
	mapClient DirectionMapClient
	legs      repository.LegStore
}

func NewDirectionService(mapClient DirectionMapClient, legs repository.LegStore) *DirectionService {
	return &DirectionService{mapClient: mapClient, legs: legs}
}

func (srv *DirectionService) Route(ctx context.Context, req request.DirectionRequest) (*tripmodel.Leg, error) {
	vehicle, err := req.Vehicle.Normalized()
	if err != nil {
		return nil, err
	}
	req.Vehicle = vehicle

	fromLat, fromLng, err := firstPoint(req.Origin)
	if err != nil {
		return nil, err
	}
	toLat, toLng, err := lastPoint(req.Destination)
	if err != nil {
		return nil, err
	}

	if !req.Alternatives {
		cached, err := srv.legs.Find(ctx, fromLat, fromLng, toLat, toLng, req.Vehicle)
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

	leg, err := locationLegFromRoute(req, fromLat, fromLng, toLat, toLng, raw.Routes[0])
	if err != nil {
		return nil, err
	}
	if err := srv.legs.Upsert(ctx, leg); err != nil {
		return nil, err
	}
	return leg, nil
}

func locationLegFromRoute(req request.DirectionRequest, fromLat, fromLng, toLat, toLng float64, route response.Route) (*tripmodel.Leg, error) {
	distanceM := 0.0
	durationS := 0.0
	steps := make([]tripmodel.RouteStep, 0)
	for _, routeLeg := range route.Legs {
		distanceM += float64(routeLeg.Distance.Value)
		durationS += float64(routeLeg.Duration.Value)
		for _, step := range routeLeg.Steps {
			steps = append(steps, tripmodel.RouteStep{
				Instruction: step.HTMLInstructions,
				Maneuver:    step.Maneuver,
				DistanceM:   float64(step.Distance.Value),
				DurationS:   float64(step.Duration.Value),
				Polyline:    step.Polyline.Points,
				StartLat:    step.StartLocation.Lat,
				StartLng:    step.StartLocation.Lng,
				EndLat:      step.EndLocation.Lat,
				EndLng:      step.EndLocation.Lng,
				TravelMode:  step.TravelMode,
			})
		}
	}
	leg := &tripmodel.Leg{
		FromLat:        fromLat,
		FromLng:        fromLng,
		ToLat:          toLat,
		ToLng:          toLng,
		Vehicle:        req.Vehicle,
		DistanceM:      distanceM,
		DurationS:      durationS,
		Polyline:       route.OverviewPolyline.Points,
		Steps:          steps,
		Source:         enum.LegSourceDirection,
		LastComputedAt: time.Now().UTC(),
	}
	return leg, nil
}
