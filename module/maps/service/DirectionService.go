package service

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/maps/model"
	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

const defaultVehicle = "motorcycle"

type DirectionMapClient interface {
	Direction(ctx context.Context, req request.DirectionRequest) (*response.DirectionResponse, error)
}

type LocationLegStore interface {
	Find(ctx context.Context, origin, destination, vehicle string) (*model.LocationLeg, error)
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
	if req.Vehicle == "" {
		req.Vehicle = defaultVehicle
	}

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
	for _, leg := range route.Legs {
		distanceM += leg.Distance.Value
		durationS += leg.Duration.Value
	}
	return &model.LocationLeg{
		Origin:      req.Origin,
		Destination: req.Destination,
		Vehicle:     req.Vehicle,
		DistanceM:   distanceM,
		DurationS:   durationS,
		Polyline:    route.OverviewPolyline.Points,
		ComputedAt:  time.Now().UTC(),
	}
}
