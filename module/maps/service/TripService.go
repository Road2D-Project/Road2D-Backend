package service

import (
	"Road-To-Destination-BE/utils/enum"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"Road-To-Destination-BE/module/maps/model"
	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
	tripmodel "Road-To-Destination-BE/module/trip/model"
)

const minTripPoints = 10

var (
	ErrTooFewTripPoints  = errors.New("trip requires at least 10 coordinates (origin, waypoints, destination)")
	ErrRoundtripSameEnds = errors.New("roundtrip origin and destination must be different")
)

type TripMapClient interface {
	Trip(ctx context.Context, req request.TripRequest) (*response.TripResponse, error)
}

type TripService struct {
	mapClient TripMapClient
	legs      repository.LegStore
}

func NewTripService(mapClient TripMapClient, legs repository.LegStore) *TripService {
	return &TripService{mapClient: mapClient, legs: legs}
}

func (srv *TripService) Optimize(ctx context.Context, req request.TripRequest) (*model.Trip, error) {
	if req.PointCount() < minTripPoints {
		return nil, ErrTooFewTripPoints
	}
	roundtrip := req.RoundtripValue()
	if roundtrip && strings.TrimSpace(req.Origin) != "" && strings.TrimSpace(req.Origin) == strings.TrimSpace(req.Destination) {
		return nil, ErrRoundtripSameEnds
	}
	vehicle, err := req.Vehicle.NormalizedOr(enum.CAR)
	if err != nil {
		return nil, err
	}
	req.Vehicle = vehicle

	raw, err := srv.mapClient.Trip(ctx, req)
	if err != nil {
		return nil, err
	}
	out := tripFromGoong(req, roundtrip, raw)
	if err := srv.cacheTripLegs(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func (srv *TripService) cacheTripLegs(ctx context.Context, out *model.Trip) error {
	if srv.legs == nil || out == nil || len(out.Trips) == 0 {
		return nil
	}
	order := out.VisitOrder
	routeLegs := out.Trips[0].Legs
	limit := len(routeLegs)
	if len(order)-1 < limit {
		limit = len(order) - 1
	}
	for i := 0; i < limit; i++ {
		steps := make([]tripmodel.RouteStep, 0, len(routeLegs[i].Steps))
		for _, step := range routeLegs[i].Steps {
			steps = append(steps, tripmodel.RouteStep{
				Instruction: step.Instruction,
				Maneuver:    step.Maneuver,
				DistanceM:   step.Distance,
				DurationS:   step.Duration,
				Polyline:    step.Geometry,
			})
		}
		leg := &tripmodel.Leg{
			FromLat:        order[i].Location.Lat,
			FromLng:        order[i].Location.Lng,
			ToLat:          order[i+1].Location.Lat,
			ToLng:          order[i+1].Location.Lng,
			Vehicle:        out.Vehicle,
			DistanceM:      routeLegs[i].Distance,
			DurationS:      routeLegs[i].Duration,
			Polyline:       "",
			Source:         enum.LegSourceTrip,
			LastComputedAt: out.ComputedAt,
		}
		if err := leg.SetSteps(steps); err != nil {
			return err
		}
		if err := srv.legs.Upsert(ctx, leg); err != nil {
			return err
		}
	}
	return nil
}

func tripFromGoong(req request.TripRequest, roundtrip bool, raw *response.TripResponse) *model.Trip {
	trips := make([]model.TripRoute, 0, len(raw.Trips))
	for _, trip := range raw.Trips {
		trips = append(trips, model.TripRoute{
			Distance:   trip.Distance,
			Duration:   trip.Duration,
			Geometry:   trip.Geometry,
			Weight:     trip.Weight,
			WeightName: trip.WeightName,
			Legs:       mapTripLegs(trip.Legs),
		})
	}
	stops := mapTripStops(raw.Waypoints)
	return &model.Trip{
		Origin:      req.Origin,
		Destination: req.Destination,
		Vehicle:     req.Vehicle,
		Roundtrip:   roundtrip,
		Code:        raw.Code,
		Trips:       trips,
		Waypoints:   stops,
		VisitOrder:  visitOrder(stops),
		ComputedAt:  time.Now().UTC(),
	}
}

func mapTripLegs(legs []response.TripLeg) []model.TripLeg {
	out := make([]model.TripLeg, 0, len(legs))
	for _, leg := range legs {
		out = append(out, model.TripLeg{
			Distance: leg.Distance,
			Duration: leg.Duration,
			Weight:   leg.Weight,
			Summary:  leg.Summary,
			Steps:    mapTripSteps(leg.Steps),
		})
	}
	return out
}

func mapTripSteps(steps []response.TripStep) []model.TripStep {
	out := make([]model.TripStep, 0, len(steps))
	for _, step := range steps {
		geometry := step.Geometry
		if geometry == "" {
			geometry = step.Polyline.Points
		}
		out = append(out, model.TripStep{
			Distance:    step.Distance,
			Duration:    step.Duration,
			Weight:      step.Weight,
			Summary:     step.Summary,
			Name:        step.Name,
			Instruction: firstNonEmpty(step.HTMLInstructions, step.Name),
			Maneuver:    maneuverText(step.Maneuver),
			Geometry:    geometry,
		})
	}
	return out
}

func mapTripStops(waypoints []response.TripWaypoint) []model.TripStop {
	out := make([]model.TripStop, 0, len(waypoints))
	for i, stop := range waypoints {
		out = append(out, model.TripStop{
			InputIndex:    i,
			Distance:      stop.Distance,
			Location:      latLngFromPair(stop.Location),
			PlaceID:       stop.PlaceID,
			TripsIndex:    stop.TripsIndex,
			WaypointIndex: stop.WaypointIndex,
		})
	}
	return out
}

func visitOrder(stops []model.TripStop) []model.TripStop {
	out := append([]model.TripStop(nil), stops...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TripsIndex != out[j].TripsIndex {
			return out[i].TripsIndex < out[j].TripsIndex
		}
		return out[i].WaypointIndex < out[j].WaypointIndex
	})
	return out
}

func latLngFromPair(pair []float64) model.LatLng {
	if len(pair) < 2 {
		return model.LatLng{}
	}
	return model.LatLng{Lat: pair[0], Lng: pair[1]}
}

func maneuverText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var maneuver struct {
		Type     string `json:"type"`
		Modifier string `json:"modifier"`
	}
	if err := json.Unmarshal(raw, &maneuver); err != nil {
		return ""
	}
	if maneuver.Modifier != "" {
		return maneuver.Modifier
	}
	return maneuver.Type
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
