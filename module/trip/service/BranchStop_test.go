package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type graphFixture struct {
	records *graphRecords
	tripID  uuid.UUID
	mainID  uuid.UUID
	subID   uuid.UUID
	a       uuid.UUID
	b       uuid.UUID
	c       uuid.UUID
	d       uuid.UUID
}

func newGraphFixture() graphFixture {
	tripID := uuid.New()
	mainID := uuid.New()
	subID := uuid.New()
	a, b, c, d := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	pins := map[uuid.UUID]model.Destination{
		a: {Base: utils.Base{ID: a}, Name: "a"},
		b: {Base: utils.Base{ID: b}, Name: "b"},
		c: {Base: utils.Base{ID: c}, Name: "c"},
		d: {Base: utils.Base{ID: d}, Name: "d"},
	}
	split := b
	merge := c
	records := &graphRecords{
		destinations: pins,
		withBranches: &model.Trip{
			Base:   utils.Base{ID: tripID},
			Status: enum.TripPlanning,
			Branches: []model.TripBranch{
				{
					Base:   utils.Base{ID: mainID},
					TripID: tripID,
					Stops: []model.BranchDestination{
						stopOf(mainID, pins[a], 0),
						stopOf(mainID, pins[b], 1),
						stopOf(mainID, pins[c], 2),
					},
				},
				{
					Base:                   utils.Base{ID: subID},
					TripID:                 tripID,
					SplitFromDestinationID: &split,
					MergeToDestinationID:   &merge,
					Stops: []model.BranchDestination{
						stopOf(subID, pins[b], 0),
						stopOf(subID, pins[d], 1),
						stopOf(subID, pins[c], 2),
					},
				},
			},
		},
	}
	return graphFixture{records: records, tripID: tripID, mainID: mainID, subID: subID, a: a, b: b, c: c, d: d}
}

func stopOf(branchID uuid.UUID, pin model.Destination, order int) model.BranchDestination {
	copied := pin
	return model.BranchDestination{
		TripBranchID:  branchID,
		DestinationID: pin.ID,
		OrderInBranch: order,
		Destination:   &copied,
	}
}

func TestDeleteBranchStopRewritesThatBranch(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)

	got, err := svc.DeleteBranchStop(context.Background(), fix.tripID, fix.subID, fix.d, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(fix.records.updated) != 1 || fix.records.updated[0].ID != fix.subID {
		t.Fatalf("updated = %+v", fix.records.updated)
	}
	wrote := fix.records.updated[0]
	if len(wrote.Stops) != 2 || wrote.Stops[0].DestinationID != fix.b || wrote.Stops[1].DestinationID != fix.c {
		t.Fatalf("stops = %+v", wrote.Stops)
	}
	if wrote.Stops[0].OrderInBranch != 0 || wrote.Stops[1].OrderInBranch != 1 {
		t.Fatalf("orders = %d %d", wrote.Stops[0].OrderInBranch, wrote.Stops[1].OrderInBranch)
	}
	if wrote.SplitFromDestinationID == nil || *wrote.SplitFromDestinationID != fix.b {
		t.Fatal("split should stay on b")
	}
	if wrote.MergeToDestinationID == nil || *wrote.MergeToDestinationID != fix.c {
		t.Fatal("merge should stay on c")
	}
	if len(got.Branches) != 2 || len(got.Branches[1].Stops) != 2 || got.Branches[1].Stops[1].DestinationID != fix.c {
		t.Fatalf("response = %+v", got.Branches)
	}
}

func TestDeleteBranchStopRejectsDisconnectedGraph(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteBranchStop(context.Background(), fix.tripID, fix.mainID, fix.b, enum.TripRoleLeader)
	if err == nil {
		t.Fatal("expected the sub branch split to become disconnected")
	}
	if len(fix.records.updated) != 0 {
		t.Fatalf("updated = %+v, want none", fix.records.updated)
	}
}

func TestDeleteBranchStopUnknownIDs(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteBranchStop(context.Background(), fix.tripID, uuid.New(), fix.a, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrBranchNotFound) {
		t.Fatalf("branch: got %v", err)
	}
	_, err = svc.DeleteBranchStop(context.Background(), fix.tripID, fix.mainID, uuid.New(), enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrBranchStopNotFound) {
		t.Fatalf("stop: got %v", err)
	}
	if len(fix.records.updated) != 0 {
		t.Fatal("a missing stop must not be written")
	}
}

func TestDeleteBranchStopRefusesLockedTrip(t *testing.T) {
	fix := newGraphFixture()
	fix.records.withBranches.Status = enum.TripLocked
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteBranchStop(context.Background(), fix.tripID, fix.subID, fix.d, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("got %v", err)
	}
	if len(fix.records.updated) != 0 {
		t.Fatal("a locked trip must not be written")
	}
}

func TestUpdateBranchStopReplacesDestination(t *testing.T) {
	fix := newGraphFixture()
	e := uuid.New()
	fix.records.destinations[e] = model.Destination{Base: utils.Base{ID: e}, Name: "e"}
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.UpdateBranchStop(context.Background(), fix.tripID, fix.subID, fix.d, request.UpdateBranchStopRequest{
		DestinationID: &e,
	}, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(fix.records.updated) != 1 {
		t.Fatalf("updated %d branches", len(fix.records.updated))
	}
	stops := fix.records.updated[0].Stops
	if len(stops) != 3 || stops[0].DestinationID != fix.b || stops[1].DestinationID != e || stops[2].DestinationID != fix.c {
		t.Fatalf("stops = %+v", stops)
	}
}

func TestUpdateBranchStopMovesWithinBranch(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)
	to := 0

	_, err := svc.UpdateBranchStop(context.Background(), fix.tripID, fix.mainID, fix.c, request.UpdateBranchStopRequest{
		OrderInBranch: &to,
	}, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(fix.records.updated) != 1 || fix.records.updated[0].ID != fix.mainID {
		t.Fatalf("updated = %+v", fix.records.updated)
	}
	stops := fix.records.updated[0].Stops
	if len(stops) != 3 || stops[0].DestinationID != fix.c || stops[1].DestinationID != fix.a || stops[2].DestinationID != fix.b {
		t.Fatalf("stops = %+v", stops)
	}
	if fix.records.updated[0].SplitFromDestinationID != nil || fix.records.updated[0].MergeToDestinationID != nil {
		t.Fatal("main branch has no split or merge")
	}
}

func TestUpdateBranchStopRejectsOrderThatBreaksMerge(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)
	to := 0

	_, err := svc.UpdateBranchStop(context.Background(), fix.tripID, fix.subID, fix.c, request.UpdateBranchStopRequest{
		OrderInBranch: &to,
	}, enum.TripRoleLeader)
	if err == nil {
		t.Fatal("expected the new tail to be disconnected")
	}
	if len(fix.records.updated) != 0 {
		t.Fatal("an invalid reorder must not be written")
	}
}

func TestDeleteBranchStopOnOpenTailKeepsMergeEmpty(t *testing.T) {
	fix := newGraphFixture()
	sub := &fix.records.withBranches.Branches[1]
	sub.MergeToDestinationID = nil
	sub.Stops = []model.BranchDestination{
		stopOf(fix.subID, fix.records.destinations[fix.b], 0),
		stopOf(fix.subID, fix.records.destinations[fix.d], 1),
		stopOf(fix.subID, fix.records.destinations[fix.a], 2),
	}
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteBranchStop(context.Background(), fix.tripID, fix.subID, fix.d, enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if len(fix.records.updated) != 1 {
		t.Fatalf("updated %d branches", len(fix.records.updated))
	}
	wrote := fix.records.updated[0]
	if wrote.MergeToDestinationID != nil {
		t.Fatal("an open tail should stay without a merge")
	}
	if len(wrote.Stops) != 2 || wrote.Stops[0].DestinationID != fix.b || wrote.Stops[1].DestinationID != fix.a {
		t.Fatalf("stops = %+v", wrote.Stops)
	}
}

func TestUpdateBranchStopRequiresAField(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.UpdateBranchStop(context.Background(), fix.tripID, fix.mainID, fix.a, request.UpdateBranchStopRequest{}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrNoBranchStopUpdate) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateBranchStopUnknownDestinationAborts(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)
	missing := uuid.New()

	_, err := svc.UpdateBranchStop(context.Background(), fix.tripID, fix.subID, fix.d, request.UpdateBranchStopRequest{
		DestinationID: &missing,
	}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrDestinationNotFound) {
		t.Fatalf("got %v", err)
	}
	if len(fix.records.updated) != 0 {
		t.Fatal("an unknown pin must not be written")
	}
}
