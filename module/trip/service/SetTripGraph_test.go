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
	destinations map[uuid.UUID]model.Destination
	replaced     []model.TripBranch
	withBranches *model.Trip
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
	return g.withBranches, nil
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

	svc := NewTripBranchService(nil, nil)
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
	//records := &graphRecords{
	//	stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: planningTrip(tripID)}},
	//}
	svc := NewTripBranchService(nil, nil)

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
	//records := &graphRecords{
	//	stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: planningTrip(tripID)}},
	//	destinations:    map[uuid.UUID]model.Destination{},
	//}
	svc := NewTripBranchService(nil, nil)

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
	//records := &graphRecords{
	//	stubTripRecords: stubTripRecords{byID: map[uuid.UUID]*model.Trip{tripID: {Status: enum.TripLocked}}},
	//}
	svc := NewTripBranchService(nil, nil)

	_, err := svc.SetTripGraph(context.Background(), tripID, request.SetTripGraphRequest{
		Branches: [][]uuid.UUID{{uuid.New()}},
		OpenTail: []bool{false},
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("got %v", err)
	}
}
