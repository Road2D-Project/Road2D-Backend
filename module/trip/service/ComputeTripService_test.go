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

type stubTravelStore struct {
	stored     []model.Travel
	findErr    error
	upsertErr  error
	upserted   []model.Travel
	upsertCall int
}

func (s *stubTravelStore) FindTravelsByTrip(_ context.Context, _ uuid.UUID) ([]model.Travel, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.stored, nil
}

func (s *stubTravelStore) UpsertTravels(_ context.Context, tripID uuid.UUID, travels []model.Travel) error {
	s.upsertCall++
	if s.upsertErr != nil {
		return s.upsertErr
	}
	for _, travel := range travels {
		travel.TripID = tripID
		s.upserted = append(s.upserted, travel)
	}
	return nil
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
	travels := &stubTravelStore{}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a1, b1}, {a2, b2}})
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
	// Different pins, so both rows are written even though one route was computed.
	if len(travels.upserted) != 2 {
		t.Fatalf("upserted rows = %d, want 2", len(travels.upserted))
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
	svc := NewComputeTripService(router, &stubTravelStore{})

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b, c}})
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
	travels := &stubTravelStore{}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if !errors.Is(err, routeErr) {
		t.Fatalf("err = %v, want %v", err, routeErr)
	}
	if got != nil {
		t.Fatalf("graph = %#v, want nil", got)
	}
	if travels.upsertCall != 0 {
		t.Fatal("a failed compute must not be written")
	}
}

func TestComputeTripShortBranchSkipsRoute(t *testing.T) {
	router := &stubLegRouter{}
	svc := NewComputeTripService(router, &stubTravelStore{})

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{stop("a", 1, 1)}})
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
	svc := NewComputeTripService(router, &stubTravelStore{})

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), nil)
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

func TestComputeTripReusesFreshTravel(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	storedID := uuid.New()
	travels := &stubTravelStore{stored: []model.Travel{{
		Base:              utils.Base{ID: storedID},
		FromDestinationID: a.ID,
		ToDestinationID:   b.ID,
		Vehicle:           enum.BIKE,
		Polyline:          "cached-poly",
		DistanceM:         999,
		LastComputedAt:    time.Now().UTC().Add(-time.Minute),
	}}}
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE, Polyline: "fresh-poly"}}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 0 {
		t.Fatalf("Route calls = %d, want 0", router.callCount())
	}
	if travels.upsertCall != 0 {
		t.Fatal("reused travel must not be rewritten")
	}
	reused := (*got)[0][0]
	if reused.ID != storedID || reused.Polyline != "cached-poly" || reused.DistanceM != 999 {
		t.Fatalf("stored row was not reused: %+v", reused)
	}
}

func TestComputeTripRecomputesExpiredTravel(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	travels := &stubTravelStore{stored: []model.Travel{{
		Base:              utils.Base{ID: uuid.New()},
		FromDestinationID: a.ID,
		ToDestinationID:   b.ID,
		Vehicle:           enum.BIKE,
		Polyline:          "stale-poly",
		LastComputedAt:    time.Now().UTC().Add(-48 * time.Hour),
	}}}
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE, Polyline: "fresh-poly", DistanceM: 42}}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 1 {
		t.Fatalf("Route calls = %d, want 1", router.callCount())
	}
	if (*got)[0][0].Polyline != "fresh-poly" {
		t.Fatalf("stale row was reused: %+v", (*got)[0][0])
	}
	if len(travels.upserted) != 1 || travels.upserted[0].Polyline != "fresh-poly" {
		t.Fatalf("upserted = %+v", travels.upserted)
	}
}

func TestComputeTripKeepsFrozenTravel(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	travels := &stubTravelStore{stored: []model.Travel{{
		Base:              utils.Base{ID: uuid.New()},
		FromDestinationID: a.ID,
		ToDestinationID:   b.ID,
		Vehicle:           enum.BIKE,
		Polyline:          "frozen-poly",
		IsFrozen:          true,
		LastComputedAt:    time.Now().UTC().Add(-100 * 24 * time.Hour),
	}}}
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE, Polyline: "fresh-poly"}}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if err != nil {
		t.Fatal(err)
	}
	if router.callCount() != 0 {
		t.Fatal("a frozen travel must never be recomputed")
	}
	if travels.upsertCall != 0 {
		t.Fatal("a frozen travel must never be rewritten")
	}
	kept := (*got)[0][0]
	if !kept.IsFrozen || kept.Polyline != "frozen-poly" {
		t.Fatalf("frozen row changed: %+v", kept)
	}
}

func TestComputeTripWritesSharedPairOnce(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	c := stop("c", 3, 3)
	travels := &stubTravelStore{}
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE, Polyline: "poly"}}
	svc := NewComputeTripService(router, travels)

	// Both branches walk the same pins a→b, so one row may reach the conflict key.
	_, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}, {a, b, c}})
	if err != nil {
		t.Fatal(err)
	}
	if len(travels.upserted) != 2 {
		t.Fatalf("upserted rows = %d, want 2 (a→b once, b→c once)", len(travels.upserted))
	}
	seen := make(map[string]bool)
	for _, row := range travels.upserted {
		key := row.FromDestinationID.String() + row.ToDestinationID.String()
		if seen[key] {
			t.Fatalf("duplicate conflict key in one batch: %+v", row)
		}
		seen[key] = true
	}
}

func TestComputeTripReturnsUpsertError(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	upsertErr := errors.New("write failed")
	travels := &stubTravelStore{upsertErr: upsertErr}
	router := &stubLegRouter{leg: &model.Leg{Vehicle: enum.BIKE}}
	svc := NewComputeTripService(router, travels)

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if !errors.Is(err, upsertErr) {
		t.Fatalf("err = %v, want %v", err, upsertErr)
	}
	if got != nil {
		t.Fatalf("graph = %#v, want nil", got)
	}
}

func TestComputeTripReturnsLoadError(t *testing.T) {
	a := stop("a", 1, 1)
	b := stop("b", 2, 2)
	findErr := errors.New("read failed")
	router := &stubLegRouter{}
	svc := NewComputeTripService(router, &stubTravelStore{findErr: findErr})

	got, err := svc.ComputeTrip(context.Background(), uuid.New(), model.GraphBranch{{a, b}})
	if !errors.Is(err, findErr) {
		t.Fatalf("err = %v, want %v", err, findErr)
	}
	if got != nil {
		t.Fatalf("graph = %#v, want nil", got)
	}
	if router.callCount() != 0 {
		t.Fatal("must not route when the stored travels cannot be read")
	}
}
