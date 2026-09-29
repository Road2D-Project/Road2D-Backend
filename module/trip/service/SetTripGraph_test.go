package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// graphRecords extends the shared trip stub with the three graph operations.
type graphRecords struct {
	stubTripRecords
	destinations       map[uuid.UUID]model.Destination
	replaced           []model.TripBranch
	withBranches       *model.Trip
	updated            []model.TripBranch
	tripsByDestination map[uuid.UUID][]uuid.UUID
	deleted            []uuid.UUID
}

func (g *graphRecords) FindDestinationsByIDs(_ context.Context, ids []uuid.UUID) ([]model.Destination, error) {
	out := make([]model.Destination, 0, len(ids))
	for _, id := range ids {
		if d, ok := g.destinations[id]; ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func (g *graphRecords) ReplaceTripBranches(_ context.Context, _ uuid.UUID, branches []model.TripBranch) error {
	g.replaced = branches
	return nil
}

func (g *graphRecords) FindTripWithBranches(context.Context, uuid.UUID) (*model.Trip, error) {
	if g.withBranches == nil {
		return nil, repository.ErrTripNotFound
	}
	return g.withBranches, nil
}

func (g *graphRecords) ListTripIDsByDestination(_ context.Context, destinationID uuid.UUID) ([]uuid.UUID, error) {
	if g.tripsByDestination == nil {
		return []uuid.UUID{}, nil
	}
	ids := g.tripsByDestination[destinationID]
	if ids == nil {
		return []uuid.UUID{}, nil
	}
	return ids, nil
}

func (g *graphRecords) DeleteDestination(_ context.Context, destinationID uuid.UUID) error {
	if _, ok := g.destinations[destinationID]; !ok {
		return repository.ErrDestinationNotFound
	}
	delete(g.destinations, destinationID)
	g.deleted = append(g.deleted, destinationID)
	return nil
}

func (g *graphRecords) UpdateBranchStops(_ context.Context, branch model.TripBranch) error {
	g.updated = append(g.updated, branch)
	if g.withBranches == nil {
		return nil
	}
	for i := range g.withBranches.Branches {
		if g.withBranches.Branches[i].ID != branch.ID {
			continue
		}
		g.withBranches.Branches[i].SplitFromDestinationID = branch.SplitFromDestinationID
		g.withBranches.Branches[i].MergeToDestinationID = branch.MergeToDestinationID
		g.withBranches.Branches[i].Stops = branch.Stops
		for j := range g.withBranches.Branches[i].Stops {
			id := g.withBranches.Branches[i].Stops[j].DestinationID
			if d, ok := g.destinations[id]; ok {
				copied := d
				g.withBranches.Branches[i].Stops[j].Destination = &copied
			}
		}
	}
	return nil
}

func planningTrip(id uuid.UUID) *model.Trip {
	return &model.Trip{Status: enum.TripPlanning}
}

func TestSetTripGraphReplacesBranches(t *testing.T) {
	tripID := uuid.New()
	d1, d2, d3 := uuid.New(), uuid.New(), uuid.New()
	records := &graphRecords{
		stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: planningTrip(tripID)}},
		destinations: map[uuid.UUID]model.Destination{
			d1: {Base: utils.Base{ID: d1}, Name: "d1"},
			d2: {Base: utils.Base{ID: d2}, Name: "d2"},
			d3: {Base: utils.Base{ID: d3}, Name: "d3"},
		},
		withBranches: &model.Trip{Status: enum.TripPlanning},
	}

	svc := NewTripBranchService(records, records)
	_, err := svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{d1, d2}, {d2, d3, d1}},
		OpenTail: []bool{false, false},
	}, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(records.replaced) != 2 {
		t.Fatalf("replaced %d branches, want 2", len(records.replaced))
	}
	sub := records.replaced[1]
	if sub.SplitFromDestinationID == nil || *sub.SplitFromDestinationID != d2 {
		t.Fatal("sub branch should split at d2")
	}
	if sub.MergeToDestinationID == nil || *sub.MergeToDestinationID != d1 {
		t.Fatal("sub branch should merge at d1")
	}
}

func TestSetTripGraphRejectsBadShape(t *testing.T) {
	tripID := uuid.New()
	records := &graphRecords{
		stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: planningTrip(tripID)}},
	}
	svc := NewTripBranchService(records, records)

	// The main branch is never open ended.
	_, err := svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{uuid.New()}},
		OpenTail: []bool{true},
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("open main: got %v", err)
	}

	// A sub branch needs both ends.
	_, err = svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{uuid.New()}, {uuid.New()}},
		OpenTail: []bool{false, false},
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("short sub branch: got %v", err)
	}
}

func TestSetTripGraphUnknownDestinationAborts(t *testing.T) {
	tripID := uuid.New()
	records := &graphRecords{
		stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: planningTrip(tripID)}},
		destinations:    map[uuid.UUID]model.Destination{},
	}
	svc := NewTripBranchService(records, records)

	_, err := svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{uuid.New(), uuid.New()}},
		OpenTail: []bool{false},
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrDestinationNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestSetTripGraphRequiresPlanningStatus(t *testing.T) {
	tripID := uuid.New()
	records := &graphRecords{
		stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: {Status: enum.TripLocked}}},
	}
	svc := NewTripBranchService(records, records)

	_, err := svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{uuid.New()}},
		OpenTail: []bool{false},
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTripGraphReturnsStoredBranches(t *testing.T) {
	tripID := uuid.New()
	d1, d2 := uuid.New(), uuid.New()
	branchID := uuid.New()
	records := &graphRecords{
		withBranches: &model.Trip{
			Base:   utils.Base{ID: tripID},
			Name:   "ride",
			Status: enum.TripLocked,
			Branches: []model.TripBranch{{
				Base:                   utils.Base{ID: branchID},
				Label:                  "main",
				SplitFromDestinationID: nil,
				Stops: []model.BranchDestination{
					{
						DestinationID: d1,
						OrderInBranch: 0,
						Destination:   &model.Destination{Base: utils.Base{ID: d1}, Name: "start", Lat: 10.1, Lng: 106.2},
					},
					{
						DestinationID: d2,
						OrderInBranch: 1,
						Destination:   &model.Destination{Base: utils.Base{ID: d2}, Name: "end", Lat: 10.3, Lng: 106.4},
					},
				},
			}},
		},
	}

	got, err := NewTripBranchService(records, records).GetTripGraph(context.Background(), tripID, enum.TripRoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != tripID || got.Name != "ride" || got.Status != enum.TripLocked {
		t.Fatalf("trip = %+v", got.TripResponse)
	}
	if got.MyRole == nil || *got.MyRole != enum.TripRoleMember {
		t.Fatalf("role = %v", got.MyRole)
	}
	if len(got.Branches) != 1 || len(got.Branches[0].Stops) != 2 {
		t.Fatalf("branches = %+v", got.Branches)
	}
	stop := got.Branches[0].Stops[0]
	if stop.DestinationID != d1 || stop.Name != "start" || stop.Lat != 10.1 || stop.OrderInBranch != 0 {
		t.Fatalf("first stop = %+v", stop)
	}
	if got.Branches[0].MergeToDestinationID != nil {
		t.Fatal("main branch has no merge")
	}
}

func TestGetTripGraphMissingTrip(t *testing.T) {
	records := &graphRecords{}
	_, err := NewTripBranchService(records, records).GetTripGraph(context.Background(), uuid.New(), enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrTripNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTripGraphEmptyStillReturnsTrip(t *testing.T) {
	tripID := uuid.New()
	records := &graphRecords{
		withBranches: &model.Trip{Base: utils.Base{ID: tripID}, Status: enum.TripPlanning},
	}
	got, err := NewTripBranchService(records, records).GetTripGraph(context.Background(), tripID, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if got.Branches == nil || len(got.Branches) != 0 {
		t.Fatalf("branches = %#v", got.Branches)
	}
}
