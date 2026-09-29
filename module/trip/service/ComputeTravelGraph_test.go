package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func TestComputeTravelGraphRoutesRequestDestinations(t *testing.T) {
	a := stop("a", 1, 2)
	b := stop("b", 3, 4)
	c := stop("c", 5, 6)
	other := stop("other", 9, 9)
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE, Polyline: "req", DistanceM: 8}}
	travels := &stubTravelStore{}
	graphs := &stubTripGraphs{
		trip: &model.Trip{
			Status: enum.TripPlanning,
			Branches: []model.TripBranch{{
				Stops: []model.BranchDestination{{Destination: &other, OrderInBranch: 0}},
			}},
		},
		destinations: map[uuid.UUID]model.Destination{
			a.ID: a,
			b.ID: b,
			c.ID: c,
		},
	}
	svc := NewComputeTripService(router, travels, nil, nil, graphs)

	got, err := svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{a.ID, b.ID}, {b.ID, c.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 2 {
		t.Fatalf("Route calls = %d, want 2", router.callCount())
	}
	if len(travels.upserted) != 2 {
		t.Fatalf("upserted = %+v", travels.upserted)
	}
	if travels.upserted[0].FromDestinationID != a.ID || travels.upserted[0].ToDestinationID != b.ID {
		t.Fatalf("first hop = %s -> %s", travels.upserted[0].FromDestinationID, travels.upserted[0].ToDestinationID)
	}
	if travels.upserted[1].FromDestinationID != b.ID || travels.upserted[1].ToDestinationID != c.ID {
		t.Fatalf("second hop = %s -> %s", travels.upserted[1].FromDestinationID, travels.upserted[1].ToDestinationID)
	}
	if got == nil || len(*got) != 2 || (*got)[0][0].FromDestinationID != a.ID || (*got)[1][0].ToDestinationID != c.ID {
		t.Fatalf("graph = %#v", got)
	}
}

func TestComputeTravelGraphRejectsBadShapeBeforeLookup(t *testing.T) {
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE}}
	graphs := &stubTripGraphs{err: repository.ErrTripNotFound}
	svc := NewComputeTripService(router, &stubTravelStore{}, nil, nil, graphs)

	_, err := svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{uuid.New()}})
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("short branch: got %v", err)
	}
	repeated := uuid.New()
	_, err = svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{repeated, repeated}})
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("duplicate: got %v", err)
	}
	_, err = svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{uuid.New(), uuid.Nil}})
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("nil id: got %v", err)
	}
	if router.callCount() != 0 {
		t.Fatal("a bad graph must not call Route")
	}
}

func TestComputeTravelGraphUnknownDestinationAborts(t *testing.T) {
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE}}
	svc := NewComputeTripService(router, &stubTravelStore{}, nil, nil, &stubTripGraphs{
		trip:         &model.Trip{Status: enum.TripPlanning},
		destinations: map[uuid.UUID]model.Destination{},
	})

	_, err := svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{uuid.New(), uuid.New()}})
	if !errors.Is(err, repository.ErrDestinationNotFound) {
		t.Fatalf("got %v", err)
	}
	if router.callCount() != 0 {
		t.Fatal("an unknown destination must not call Route")
	}
}

func TestComputeTravelGraphRefusesLockedTrip(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE}}
	travels := &stubTravelStore{}
	svc := NewComputeTripService(router, travels, nil, nil, &stubTripGraphs{
		trip: &model.Trip{Status: enum.TripLocked},
		destinations: map[uuid.UUID]model.Destination{
			a.ID: a,
			b.ID: b,
		},
	})

	_, err := svc.ComputeTravelGraph(context.Background(), uuid.New(), [][]uuid.UUID{{a.ID, b.ID}})
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("got %v", err)
	}
	if router.callCount() != 0 || travels.upsertCall != 0 {
		t.Fatal("a locked trip must not be routed")
	}
}
