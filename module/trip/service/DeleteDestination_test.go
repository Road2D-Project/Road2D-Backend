package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func TestDeleteDestinationRemovesStopThenDeletesPin(t *testing.T) {
	fix := newGraphFixture()
	fix.records.tripsByDestination = map[uuid.UUID][]uuid.UUID{fix.d: {fix.tripID}}
	svc := NewTripBranchService(fix.records, fix.records)

	affected, err := svc.DeleteDestination(context.Background(), fix.d)
	if err != nil {
		t.Fatal(err)
	}
	if len(affected) != 1 || affected[0] != fix.tripID {
		t.Fatalf("affected = %v", affected)
	}
	if len(fix.records.deleted) != 1 || fix.records.deleted[0] != fix.d {
		t.Fatalf("deleted = %v", fix.records.deleted)
	}
	if len(fix.records.updated) != 1 || fix.records.updated[0].ID != fix.subID {
		t.Fatalf("updated = %+v", fix.records.updated)
	}
	stops := fix.records.updated[0].Stops
	if len(stops) != 2 || stops[0].DestinationID != fix.b || stops[1].DestinationID != fix.c {
		t.Fatalf("stops = %+v", stops)
	}
}

func TestDeleteDestinationLeavesGraphWhenRemovalDisconnects(t *testing.T) {
	fix := newGraphFixture()
	fix.records.tripsByDestination = map[uuid.UUID][]uuid.UUID{fix.b: {fix.tripID}}
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteDestination(context.Background(), fix.b)
	if err == nil {
		t.Fatal("expected the graph to stay connected")
	}
	if len(fix.records.updated) != 0 || len(fix.records.deleted) != 0 {
		t.Fatal("a disconnected graph must not be written or deleted")
	}
}

func TestDeleteDestinationRejectsPinThatIsNotEditing(t *testing.T) {
	fix := newGraphFixture()
	pin := fix.records.destinations[fix.d]
	pin.Status = enum.Confirm
	fix.records.destinations[fix.d] = pin
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteDestination(context.Background(), fix.d)
	if !errors.Is(err, repository.ErrDestinationNotEditing) {
		t.Fatalf("got %v", err)
	}
	if len(fix.records.deleted) != 0 {
		t.Fatal("a confirmed pin must stay")
	}
}

func TestDeleteDestinationMissingPin(t *testing.T) {
	fix := newGraphFixture()
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteDestination(context.Background(), uuid.New())
	if !errors.Is(err, repository.ErrDestinationNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteDestinationRefusesLockedTrip(t *testing.T) {
	fix := newGraphFixture()
	fix.records.withBranches.Status = enum.TripLocked
	fix.records.tripsByDestination = map[uuid.UUID][]uuid.UUID{fix.d: {fix.tripID}}
	svc := NewTripBranchService(fix.records, fix.records)

	_, err := svc.DeleteDestination(context.Background(), fix.d)
	if !errors.Is(err, repository.ErrDestinationInUse) {
		t.Fatalf("got %v", err)
	}
	if len(fix.records.updated) != 0 || len(fix.records.deleted) != 0 {
		t.Fatal("a locked trip must keep the pin")
	}
}

func TestDeleteDestinationUnusedPin(t *testing.T) {
	fix := newGraphFixture()
	free := uuid.New()
	fix.records.destinations[free] = model.Destination{Base: utils.Base{ID: free}, Name: "free", Status: enum.Editing}
	svc := NewTripBranchService(fix.records, fix.records)

	affected, err := svc.DeleteDestination(context.Background(), free)
	if err != nil {
		t.Fatal(err)
	}
	if len(affected) != 0 {
		t.Fatalf("affected = %v", affected)
	}
	if len(fix.records.deleted) != 1 || fix.records.deleted[0] != free {
		t.Fatalf("deleted = %v", fix.records.deleted)
	}
	if len(fix.records.updated) != 0 {
		t.Fatal("an unused pin has no branch to rewrite")
	}
}
