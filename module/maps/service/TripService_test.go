package service

import (
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
)

type stubTripClient struct {
	calls         int
	lastVehicle   enum.Vehicle
	lastRoundtrip string
	lastSteps     string
	out           *response.TripResponse
	err           error
}

func (s *stubTripClient) Trip(_ context.Context, req request.TripRequest) (*response.TripResponse, error) {
	s.calls++
	s.lastVehicle = req.Vehicle
	if req.RoundtripValue() {
		s.lastRoundtrip = "true"
	} else {
		s.lastRoundtrip = "false"
	}
	if req.StepsValue() {
		s.lastSteps = "true"
	} else {
		s.lastSteps = "false"
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.out, nil
}

func sampleTripRequest() request.TripRequest {
	return request.TripRequest{
		Origin:      "21.03931,105.83997",
		Destination: "21.01343,105.79855",
		Waypoints: "21.03303,105.79131;21.01765,105.80350;21.00755,105.81105;" +
			"20.99834,105.79148;21.00507,105.78814;21.01917,105.78822;" +
			"21.02694,105.79466;21.01276,105.80256;21.02061,105.78925;21.01028,105.78942",
	}
}

func sampleGoongTrip() *response.TripResponse {
	return &response.TripResponse{
		Code: "Ok",
		Trips: []response.GoongTrip{{
			Distance:   33343.4,
			Duration:   10065.2,
			Geometry:   "wfl_Coz~dS",
			Weight:     10065.2,
			WeightName: "routability",
			Legs: []response.TripLeg{{
				Distance: 6795.3,
				Duration: 1259.8,
				Weight:   1259.8,
				Summary:  "",
				Steps:    []response.TripStep{},
			}},
		}},
		Waypoints: []response.TripWaypoint{{
			Distance:      5.033847,
			Location:      []float64{21.039316, 105.839922},
			PlaceID:       "place-origin",
			TripsIndex:    0,
			WaypointIndex: 0,
		}, {
			Distance:      2.754927,
			Location:      []float64{21.033061, 105.791325},
			PlaceID:       "place-wp",
			TripsIndex:    0,
			WaypointIndex: 1,
		}},
	}
}

func TestTripServiceDefaultVehicleAndRoundtrip(t *testing.T) {
	stub := &stubTripClient{out: sampleGoongTrip()}
	out, err := NewTripService(stub).Optimize(context.Background(), sampleTripRequest())
	if err != nil {
		t.Fatal(err)
	}
	if out.Vehicle != enum.CAR {
		t.Fatalf("default vehicle: %s", out.Vehicle)
	}
	if !out.Roundtrip {
		t.Fatal("roundtrip should default true")
	}
	if stub.lastVehicle != enum.CAR || stub.lastRoundtrip != "true" {
		t.Fatalf("goong vehicle=%s roundtrip=%s", stub.lastVehicle, stub.lastRoundtrip)
	}
	if stub.lastSteps != "true" {
		t.Fatalf("steps should default true, got %s", stub.lastSteps)
	}
	if out.Code != "Ok" || len(out.Trips) != 1 {
		t.Fatalf("unexpected trip: %+v", out)
	}
	if out.Trips[0].Distance != 33343.4 || out.Trips[0].WeightName != "routability" {
		t.Fatalf("unexpected route: %+v", out.Trips[0])
	}
	if out.Waypoints[0].Location.Lat != 21.039316 || out.Waypoints[0].Location.Lng != 105.839922 {
		t.Fatalf("waypoint should be lat,lng: %+v", out.Waypoints[0])
	}
	if out.Waypoints[1].WaypointIndex != 1 {
		t.Fatalf("waypoint_index: %d", out.Waypoints[1].WaypointIndex)
	}
}

func TestTripServiceRejectsTooFewPoints(t *testing.T) {
	_, err := NewTripService(&stubTripClient{}).Optimize(context.Background(), request.TripRequest{
		Origin:      "21.0,105.8",
		Destination: "21.1,105.8",
		Waypoints:   "21.02,105.79",
	})
	if !errors.Is(err, ErrTooFewTripPoints) {
		t.Fatalf("got %v", err)
	}
}

func TestTripServiceRejectsRoundtripSameEnds(t *testing.T) {
	req := sampleTripRequest()
	req.Destination = req.Origin
	_, err := NewTripService(&stubTripClient{}).Optimize(context.Background(), req)
	if !errors.Is(err, ErrRoundtripSameEnds) {
		t.Fatalf("got %v", err)
	}
}

func TestTripServiceAllowsSameEndsWhenNotRoundtrip(t *testing.T) {
	req := sampleTripRequest()
	req.Destination = req.Origin
	roundtrip := false
	req.Roundtrip = &roundtrip
	stub := &stubTripClient{out: sampleGoongTrip()}
	if _, err := NewTripService(stub).Optimize(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if stub.lastRoundtrip != "false" {
		t.Fatalf("roundtrip=%s", stub.lastRoundtrip)
	}
}

func TestTripServiceRejectsUnknownVehicle(t *testing.T) {
	req := sampleTripRequest()
	req.Vehicle = enum.Vehicle(99)
	_, err := NewTripService(&stubTripClient{}).Optimize(context.Background(), req)
	if !errors.Is(err, enum.ErrUnsupportedVehicle) {
		t.Fatalf("got %v", err)
	}
}

func TestTripServiceVisitOrderFollowsWaypointIndex(t *testing.T) {
	raw := sampleGoongTrip()
	raw.Waypoints = []response.TripWaypoint{
		{Location: []float64{21.00, 105.79}, PlaceID: "input-0", WaypointIndex: 2},
		{Location: []float64{21.01, 105.79}, PlaceID: "input-1", WaypointIndex: 0},
		{Location: []float64{21.02, 105.79}, PlaceID: "input-2", WaypointIndex: 1},
	}
	out, err := NewTripService(&stubTripClient{out: raw}).Optimize(context.Background(), sampleTripRequest())
	if err != nil {
		t.Fatal(err)
	}
	if out.Waypoints[0].InputIndex != 0 || out.Waypoints[0].WaypointIndex != 2 {
		t.Fatalf("waypoints stay in input order: %+v", out.Waypoints[0])
	}
	if out.VisitOrder[0].InputIndex != 1 || out.VisitOrder[1].InputIndex != 2 || out.VisitOrder[2].InputIndex != 0 {
		t.Fatalf("visit_order=%+v", out.VisitOrder)
	}
}

func TestTripServiceMapsOSRMManeuver(t *testing.T) {
	raw := sampleGoongTrip()
	raw.Trips[0].Legs[0].Steps = []response.TripStep{{
		Distance: 41,
		Duration: 10,
		Name:     "Trần Cung",
		Maneuver: []byte(`{"type":"turn","modifier":"left"}`),
	}}
	out, err := NewTripService(&stubTripClient{out: raw}).Optimize(context.Background(), sampleTripRequest())
	if err != nil {
		t.Fatal(err)
	}
	step := out.Trips[0].Legs[0].Steps[0]
	if step.Maneuver != "left" || step.Instruction != "Trần Cung" {
		t.Fatalf("step=%+v", step)
	}
}
