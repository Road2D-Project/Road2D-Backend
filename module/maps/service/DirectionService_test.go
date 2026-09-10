package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
	"Road-To-Destination-BE/module/utils/enum"
)

type stubDirectionClient struct {
	calls       int
	lastVehicle enum.Vehicle
	out         *response.DirectionResponse
}

func (s *stubDirectionClient) Direction(_ context.Context, req request.DirectionRequest) (*response.DirectionResponse, error) {
	s.calls++
	s.lastVehicle = req.Vehicle
	return s.out, nil
}

func TestDirectionServiceCachesLocationLeg(t *testing.T) {
	stub := &stubDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{{
					Distance: response.TextValue{Value: 1000},
					Duration: response.TextValue{Value: 120},
					Steps: []response.Step{{
						Distance:          response.TextValue{Value: 1000},
						Duration:          response.TextValue{Value: 120},
						HTMLInstructions: "Bắt đầu đi từ Trần Cung",
						Maneuver:          "left",
						Polyline:          response.OverviewPolyline{Points: "abc"},
						StartLocation:     response.LatLng{Lat: 21.04663, Lng: 105.79022},
						EndLocation:       response.LatLng{Lat: 21.04667, Lng: 105.79022},
						TravelMode:        "DRIVING",
					}},
				}},
				OverviewPolyline: response.OverviewPolyline{Points: "abc"},
			}},
		},
	}
	store := repository.NewLocationLegMemoryStore()
	svc := NewDirectionService(stub, store)
	ctx := context.Background()
	req := request.DirectionRequest{
		Origin:      "10.77,106.70",
		Destination: "10.78,106.71",
		Vehicle:     enum.BIKE,
	}

	first, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.DistanceM != 1000 || first.Polyline != "abc" {
		t.Fatalf("unexpected first leg: %+v", first)
	}
	if len(first.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(first.Steps))
	}
	if first.Steps[0].Instruction != "Bắt đầu đi từ Trần Cung" || first.Steps[0].Maneuver != "left" {
		t.Fatalf("unexpected step: %+v", first.Steps[0])
	}

	second, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if stub.calls != 1 {
		t.Fatalf("expected cache hit, Goong called %d times", stub.calls)
	}
	if time.Since(second.ComputedAt) < 0 {
		t.Fatal("computed_at should be set")
	}

	cached, err := store.Find(ctx, req.Origin, req.Destination, req.Vehicle)
	if err != nil || cached == nil {
		t.Fatalf("expected stored LocationLeg, err=%v", err)
	}
}

func TestLocationLegMemoryStoreMiss(t *testing.T) {
	store := repository.NewLocationLegMemoryStore()
	got, err := store.Find(context.Background(), "a", "b", enum.CAR)
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected miss, got %+v", got)
	}
}

func TestDirectionServiceDefaultVehicle(t *testing.T) {
	stub := &stubDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{{
					Distance: response.TextValue{Value: 1},
					Duration: response.TextValue{Value: 1},
				}},
			}},
		},
	}
	store := repository.NewLocationLegMemoryStore()
	svc := NewDirectionService(stub, store)
	leg, err := svc.Route(context.Background(), request.DirectionRequest{
		Origin:      "1,2",
		Destination: "3,4",
	})
	if err != nil {
		t.Fatal(err)
	}
	if leg.Vehicle != enum.BIKE {
		t.Fatalf("default vehicle: %s", leg.Vehicle)
	}
}

func TestDirectionServiceAliasesMotorbikeToBike(t *testing.T) {
	stub := &stubDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{{
					Distance: response.TextValue{Value: 1},
					Duration: response.TextValue{Value: 1},
				}},
			}},
		},
	}
	store := repository.NewLocationLegMemoryStore()
	svc := NewDirectionService(stub, store)
	motorbike, err := enum.ParseVehicle("motorbike")
	if err != nil {
		t.Fatal(err)
	}
	leg, err := svc.Route(context.Background(), request.DirectionRequest{
		Origin:      "1,2",
		Destination: "3,4",
		Vehicle:     motorbike,
	})
	if err != nil {
		t.Fatal(err)
	}
	if leg.Vehicle != enum.BIKE {
		t.Fatalf("aliased vehicle: %s", leg.Vehicle)
	}
	if stub.lastVehicle != enum.BIKE {
		t.Fatalf("goong vehicle: %s", stub.lastVehicle)
	}
}

func TestDirectionServiceRejectsUnknownVehicle(t *testing.T) {
	svc := NewDirectionService(&stubDirectionClient{}, repository.NewLocationLegMemoryStore())
	_, err := svc.Route(context.Background(), request.DirectionRequest{
		Origin:      "1,2",
		Destination: "3,4",
		Vehicle:     enum.Vehicle(99),
	})
	if !errors.Is(err, enum.ErrUnsupportedVehicle) {
		t.Fatalf("got %v", err)
	}
}

func TestDirectionServiceConcatenatesStepsAcrossLegs(t *testing.T) {
	stub := &stubDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{
					{
						Distance: response.TextValue{Value: 5},
						Duration: response.TextValue{Value: 1},
						Steps: []response.Step{{
							HTMLInstructions: "Bắt đầu đi từ Trần Cung",
							Maneuver:          "left",
							Distance:          response.TextValue{Value: 5},
							Duration:          response.TextValue{Value: 1},
							Polyline:          response.OverviewPolyline{Points: "aaa"},
						}},
					},
					{
						Distance: response.TextValue{Value: 0},
						Duration: response.TextValue{Value: 0},
						Steps: []response.Step{{
							HTMLInstructions: "Bạn đã đến điểm đích",
							Polyline:          response.OverviewPolyline{Points: "bbb"},
						}},
					},
				},
			}},
		},
	}
	svc := NewDirectionService(stub, repository.NewLocationLegMemoryStore())
	leg, err := svc.Route(context.Background(), request.DirectionRequest{
		Origin:      "1,2",
		Destination: "3,4",
		Vehicle:     enum.BIKE,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(leg.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(leg.Steps))
	}
	if leg.Steps[1].Instruction != "Bạn đã đến điểm đích" {
		t.Fatalf("unexpected last step: %+v", leg.Steps[1])
	}
}
