package service

import (
	"context"
	"testing"
	"time"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/maps/repository"
)

type stubDirectionClient struct {
	calls int
	out   *response.DirectionResponse
}

func (s *stubDirectionClient) Direction(_ context.Context, _ request.DirectionRequest) (*response.DirectionResponse, error) {
	s.calls++
	return s.out, nil
}

func TestDirectionServiceCachesLocationLeg(t *testing.T) {
	stub := &stubDirectionClient{
		out: &response.DirectionResponse{
			Routes: []response.Route{{
				Legs: []response.Leg{{
					Distance: response.TextValue{Value: 1000},
					Duration: response.TextValue{Value: 120},
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
		Vehicle:     "motorcycle",
	}

	first, err := svc.Route(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if first.DistanceM != 1000 || first.Polyline != "abc" {
		t.Fatalf("unexpected first leg: %+v", first)
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
	got, err := store.Find(context.Background(), "a", "b", "car")
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
	if leg.Vehicle != "motorcycle" {
		t.Fatalf("default vehicle: %s", leg.Vehicle)
	}
}
