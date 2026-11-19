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

type planningPolicyStore struct {
	PlanningStore
	usages []repository.DestinationUsage
	roles  map[uuid.UUID]enum.TripRole
	trips  map[uuid.UUID]*model.Trip
	pins   map[uuid.UUID]model.Destination
}

func (s planningPolicyStore) FindDestinationsByIDs(_ context.Context, ids []uuid.UUID) ([]model.Destination, error) {
	var pins []model.Destination
	for _, id := range ids {
		if pin, ok := s.pins[id]; ok {
			pins = append(pins, pin)
		}
	}
	return pins, nil
}

func (s planningPolicyStore) DestinationUsages(context.Context, uuid.UUID) ([]repository.DestinationUsage, error) {
	return s.usages, nil
}

func (s planningPolicyStore) FindTripActiveMemberRole(_ context.Context, tripID, _ uuid.UUID) (enum.TripRole, error) {
	role, ok := s.roles[tripID]
	if !ok {
		return 0, repository.ErrUserNotTripMember
	}
	return role, nil
}

func (s planningPolicyStore) FindTripByID(_ context.Context, tripID uuid.UUID) (*model.Trip, error) {
	return s.trips[tripID], nil
}

func TestDestinationPolicyStandaloneOwnerAndLegacy(t *testing.T) {
	owner, other := uuid.New(), uuid.New()
	pin := &model.Destination{Base: utils.Base{ID: uuid.New()}, CreatedByUserID: &owner}
	store := planningPolicyStore{}
	if _, err := editableDestination(t.Context(), store, owner, pin); err != nil {
		t.Fatal(err)
	}
	if err := canReadDestination(t.Context(), store, other, pin); !errors.Is(err, repository.ErrDestinationForbidden) {
		t.Fatalf("other user read: %v", err)
	}
	pin.CreatedByUserID = nil
	if err := canReadDestination(t.Context(), store, owner, pin); !errors.Is(err, repository.ErrDestinationForbidden) {
		t.Fatalf("unattributed legacy pin read: %v", err)
	}
}

func TestDestinationPolicyRequiresPermissionOnEveryTrip(t *testing.T) {
	actor, first, second := uuid.New(), uuid.New(), uuid.New()
	pin := &model.Destination{Base: utils.Base{ID: uuid.New()}, CreatedByUserID: &actor}
	store := planningPolicyStore{
		usages: []repository.DestinationUsage{{TripID: first, OnRoute: true}, {TripID: second, OnRoute: true}},
		roles:  map[uuid.UUID]enum.TripRole{first: enum.TripRoleLeader, second: enum.TripRoleAdmin},
		trips:  map[uuid.UUID]*model.Trip{first: {Status: enum.TripPlanning}, second: {Status: enum.TripPlanning}},
	}
	if err := canReadDestination(t.Context(), store, actor, pin); err != nil {
		t.Fatal(err)
	}
	if _, err := editableDestination(t.Context(), store, actor, pin); !errors.Is(err, repository.ErrDestinationForbidden) {
		t.Fatalf("admin on second trip can edit shared route pin: %v", err)
	}
	store.roles[second] = enum.TripRoleLeader
	if affected, err := editableDestination(t.Context(), store, actor, pin); err != nil || len(affected) != 2 {
		t.Fatalf("shared leader edit: affected=%v err=%v", affected, err)
	}
	store.trips[second].Status = enum.TripCompleted
	if _, err := editableDestination(t.Context(), store, actor, pin); !errors.Is(err, repository.ErrDestinationInUse) {
		t.Fatalf("completed trip did not protect shared pin: %v", err)
	}
}

func TestDestinationPolicyReaderCannotShareWithAnotherTrip(t *testing.T) {
	actor, source, target := uuid.New(), uuid.New(), uuid.New()
	pin := model.Destination{Base: utils.Base{ID: uuid.New()}}
	store := planningPolicyStore{
		usages: []repository.DestinationUsage{{TripID: source, OnRoute: true}},
		roles:  map[uuid.UUID]enum.TripRole{source: enum.TripRoleMember},
		pins:   map[uuid.UUID]model.Destination{pin.ID: pin},
	}
	if err := canReadDestination(t.Context(), store, actor, &pin); err != nil {
		t.Fatal(err)
	}
	if _, err := usableDestinations(t.Context(), store, actor, target, []uuid.UUID{pin.ID}); !errors.Is(err, repository.ErrDestinationForbidden) {
		t.Fatalf("reader shared source pin: %v", err)
	}
	store.roles[source] = enum.TripRoleLeader
	if _, err := usableDestinations(t.Context(), store, actor, target, []uuid.UUID{pin.ID}); err != nil {
		t.Fatalf("source leader sharing: %v", err)
	}
}

func TestDestinationPolicyDraftCreatorMustRemainActive(t *testing.T) {
	actor, tripID := uuid.New(), uuid.New()
	pin := &model.Destination{Base: utils.Base{ID: uuid.New()}, CreatedByUserID: &actor}
	store := planningPolicyStore{
		usages: []repository.DestinationUsage{{TripID: tripID}},
		roles:  map[uuid.UUID]enum.TripRole{tripID: enum.TripRoleMember},
		trips:  map[uuid.UUID]*model.Trip{tripID: {Status: enum.TripPlanning}},
	}
	if _, err := editableDestination(t.Context(), store, actor, pin); err != nil {
		t.Fatal(err)
	}
	delete(store.roles, tripID)
	if err := canReadDestination(t.Context(), store, actor, pin); !errors.Is(err, repository.ErrDestinationForbidden) {
		t.Fatalf("removed creator can read attached pin: %v", err)
	}
}

func TestDestinationPolicyProtectsFrozenTravelWithoutBranch(t *testing.T) {
	actor, tripID := uuid.New(), uuid.New()
	pin := &model.Destination{Base: utils.Base{ID: uuid.New()}}
	store := planningPolicyStore{
		usages: []repository.DestinationUsage{{TripID: tripID, HasTravel: true, Frozen: true}},
		roles:  map[uuid.UUID]enum.TripRole{tripID: enum.TripRoleLeader},
		trips:  map[uuid.UUID]*model.Trip{tripID: {Status: enum.TripPlanning}},
	}
	if _, err := editableDestination(t.Context(), store, actor, pin); !errors.Is(err, repository.ErrDestinationInUse) {
		t.Fatalf("frozen history did not protect pin: %v", err)
	}
}
