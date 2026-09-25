package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"Road-To-Destination-BE/module/maps/model/request"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type stubLegRouter struct {
	mu    sync.Mutex
	calls int
	reqs  []request.DirectionRequest
	leg   *model.Leg
	err   error
}

func (s *stubLegRouter) Route(_ context.Context, req request.DirectionRequest) (*model.Leg, error) {
	s.mu.Lock()
	s.calls++
	s.reqs = append(s.reqs, req)
	s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	return s.leg, nil
}

func (s *stubLegRouter) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *stubLegRouter) requests() []request.DirectionRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]request.DirectionRequest, len(s.reqs))
	copy(out, s.reqs)
	return out
}

func stop(name string, lat, lng float64) model.Destination {
	return model.Destination{
		Base: utils.Base{ID: uuid.New()},
		Name: name,
		Lat:  lat,
		Lng:  lng,
	}
}

func TestComputeTripDedupesSharedCoordinates(t *testing.T) {
	a1 := stop("a1", 10.5, 20.25)
	b1 := stop("b1", 30, 40)
	a2 := stop("a2", 10.5, 20.25)
	b2 := stop("b2", 30, 40)
	legID := uuid.New()
	computedAt := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	router := &stubLegRouter{leg: &model.Leg{
		Base:           utils.Base{ID: legID},
		Vehicle:        enum.BIKE,
		Polyline:       "shared-poly",
		DistanceM:      1200,
		DurationS:      340,
		LastComputedAt: computedAt,
		Steps:          []model.RouteStep{{Instruction: "turn left"}},
	}}
	svc := NewComputeTripService(router)

	got, err := svc.ComputeTrip(context.Background(), model.GraphBranch{{a1, b1}, {a2, b2}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 1 {
		t.Fatalf("Route calls = %d, want 1", router.callCount())
	}
	reqs := router.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	if reqs[0].Origin != "10.5,20.25" || reqs[0].Destination != "30,40" || reqs[0].Vehicle != enum.BIKE || reqs[0].Alternatives {
		t.Fatalf("request = %+v", reqs[0])
	}
	if got == nil || len(*got) != 2 || len((*got)[0]) != 1 || len((*got)[1]) != 1 {
		t.Fatalf("graph shape = %#v", got)
	}
	left := (*got)[0][0]
	right := (*got)[1][0]
	if left.FromDestinationID != a1.ID || left.ToDestinationID != b1.ID {
		t.Fatalf("first slot ids = %s -> %s", left.FromDestinationID, left.ToDestinationID)
	}
	if right.FromDestinationID != a2.ID || right.ToDestinationID != b2.ID {
		t.Fatalf("second slot ids = %s -> %s", right.FromDestinationID, right.ToDestinationID)
	}
	if left.Polyline != "shared-poly" || right.Polyline != "shared-poly" || left.DistanceM != 1200 || right.DistanceM != 1200 {
		t.Fatalf("shared leg not copied: %+v %+v", left, right)
	}
	if left.DurationS != 340 || right.DurationS != 340 || left.Vehicle != enum.BIKE || right.Vehicle != enum.BIKE {
		t.Fatalf("vehicle or duration = %+v %+v", left, right)
	}
	if !left.LastComputedAt.Equal(computedAt) || !right.LastComputedAt.Equal(computedAt) {
		t.Fatalf("computed at = %s %s", left.LastComputedAt, right.LastComputedAt)
	}
	if left.IsFrozen || right.IsFrozen {
		t.Fatal("computed travel must stay unfrozen")
	}
	if left.LegID == nil || *left.LegID != legID || right.LegID == nil || *right.LegID != legID {
		t.Fatal("leg id was not copied onto both slots")
	}
	if left.LegID == right.LegID {
		t.Fatal("slots should not share one leg id pointer")
	}
}

func TestComputeTripOrdersLegsOnOneBranch(t *testing.T) {
	a := stop("a", 1, 2)
	b := stop("b", 3, 4)
	c := stop("c", 5, 6)
	router := &stubLegRouter{leg: &model.Leg{
		Vehicle:   enum.BIKE,
		Polyline:  "plain",
		DistanceM: 10,
	}}
	svc := NewComputeTripService(router)

	got, err := svc.ComputeTrip(context.Background(), model.GraphBranch{{a, b, c}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 2 {
		t.Fatalf("Route calls = %d, want 2", router.callCount())
	}
	if got == nil || len(*got) != 1 || len((*got)[0]) != 2 {
		t.Fatalf("graph shape = %#v", got)
	}
	first := (*got)[0][0]
	second := (*got)[0][1]
	if first.FromDestinationID != a.ID || first.ToDestinationID != b.ID {
		t.Fatalf("first leg = %s -> %s", first.FromDestinationID, first.ToDestinationID)
	}
	if second.FromDestinationID != b.ID || second.ToDestinationID != c.ID {
		t.Fatalf("second leg = %s -> %s", second.FromDestinationID, second.ToDestinationID)
	}
	if first.LegID != nil || second.LegID != nil {
		t.Fatal("leg without a database id should leave LegID empty")
	}
	if first.IsFrozen || second.IsFrozen {
		t.Fatal("computed travel must stay unfrozen")
	}
}

func TestComputeTripReturnsRouteError(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	routeErr := errors.New("goong down")
	router := &stubLegRouter{err: routeErr}
	svc := NewComputeTripService(router)

	got, err := svc.ComputeTrip(context.Background(), model.GraphBranch{{a, b}})
	if !errors.Is(err, routeErr) {
		t.Fatalf("err = %v, want %v", err, routeErr)
	}
	if got != nil {
		t.Fatalf("graph = %#v, want nil", got)
	}
}

func TestComputeTripShortBranchSkipsRoute(t *testing.T) {
	router := &stubLegRouter{}
	svc := NewComputeTripService(router)

	got, err := svc.ComputeTrip(context.Background(), model.GraphBranch{{stop("a", 1, 1)}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 0 {
		t.Fatalf("Route calls = %d, want 0", router.callCount())
	}
	if got == nil || len(*got) != 1 || (*got)[0] == nil || len((*got)[0]) != 0 {
		t.Fatalf("graph = %#v", got)
	}
}

func TestComputeTripEmptyGraphSkipsRoute(t *testing.T) {
	router := &stubLegRouter{}
	svc := NewComputeTripService(router)

	got, err := svc.ComputeTrip(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 0 {
		t.Fatalf("Route calls = %d, want 0", router.callCount())
	}
	if got == nil || len(*got) != 0 {
		t.Fatalf("graph = %#v", got)
	}
}
