package service

import (
	"context"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type planningDestinations map[uuid.UUID]model.Destination

func (pins planningDestinations) FindDestinationById(_ context.Context, id uuid.UUID) (*model.Destination, error) {
	pin, ok := pins[id]
	if !ok {
		return nil, repository.ErrDestinationNotFound
	}
	return &pin, nil
}

func (s *PlanningService) PreviewBranch(ctx context.Context, actorID, tripID uuid.UUID, points []request.ComputeBranchPoint) (*response.ComputeBranchResponse, error) {
	pins := planningDestinations{}
	err := s.transaction(ctx, func(store PlanningStore) error {
		if _, _, err := requirePlanningMember(ctx, store, actorID, tripID, false); err != nil {
			return err
		}
		var ids []uuid.UUID
		for _, point := range points {
			if point.DestinationID != nil {
				ids = append(ids, *point.DestinationID)
			}
		}
		found, err := readableDestinations(ctx, store, actorID, referencedDestinationIDs([][]uuid.UUID{ids}))
		if err != nil {
			return err
		}
		for _, pin := range found {
			pins[pin.ID] = pin
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return NewComputeTripService(s.previewRoutes, nil, pins, s.locations, nil).PreviewBranch(ctx, points)
}

// Provider requests run after the read transaction ends. A second transaction
// rechecks permissions and versions before publishing the computed travels.
func (s *PlanningService) ComputeTravels(ctx context.Context, actorID, tripID uuid.UUID, branches [][]uuid.UUID) (*model.TravelGraph, error) {
	var trip *model.Trip
	var graph model.BranchGraph
	var pins []model.Destination
	var ids []uuid.UUID
	staged := &planningTravels{}
	err := s.transaction(ctx, func(store PlanningStore) error {
		var err error
		trip, _, err = requirePlanningMember(ctx, store, actorID, tripID, true)
		if err != nil {
			return err
		}
		if trip.Status != enum.TripPlanning {
			return repository.ErrInvalidTripGraph
		}
		requested := branches
		if branches == nil {
			stored, err := store.FindTripWithBranches(ctx, tripID)
			if err != nil {
				return err
			}
			graph, err = model.GraphFromTrip(stored)
			if err != nil {
				return err
			}
			requested = make([][]uuid.UUID, len(graph))
			for i, branch := range graph {
				for _, pin := range branch {
					requested[i] = append(requested[i], pin.ID)
				}
			}
		} else if err := validateTravelBranches(branches); err != nil {
			return err
		}
		ids = referencedDestinationIDs(requested)
		pins, err = usableDestinations(ctx, store, actorID, tripID, ids)
		if err != nil {
			return err
		}
		graph, err = resolveGraph(requested, pins)
		if err != nil {
			return err
		}
		staged.stored, err = store.FindTravelsByTrip(ctx, tripID)
		return err
	})
	if err != nil {
		return nil, err
	}
	result, err := NewComputeTripService(s.routes, staged, nil, nil, nil).ComputeTrip(ctx, tripID, graph)
	if err != nil {
		return nil, err
	}
	expected := planningDestinations{}
	for _, pin := range pins {
		expected[pin.ID] = pin
	}
	err = s.transaction(ctx, func(store PlanningStore) error {
		current, _, err := requirePlanningMember(ctx, store, actorID, tripID, true)
		if err != nil {
			return err
		}
		if current.Status != enum.TripPlanning || !current.UpdatedAt.Equal(trip.UpdatedAt) {
			return repository.ErrPlanningConflict
		}
		found, err := usableDestinations(ctx, store, actorID, tripID, ids)
		if err != nil {
			return err
		}
		for _, pin := range found {
			prior := expected[pin.ID]
			if !pin.UpdatedAt.Equal(prior.UpdatedAt) || pin.Lat != prior.Lat || pin.Lng != prior.Lng {
				return repository.ErrPlanningConflict
			}
		}
		if len(staged.pending) == 0 {
			return nil
		}
		if err := store.UpsertTravels(ctx, tripID, staged.pending); err != nil {
			return err
		}
		return store.TouchTrip(ctx, tripID)
	})
	return result, err
}

// planningTravels buffers writes while the provider is running.
type planningTravels struct {
	stored  []model.Travel
	pending []model.Travel
}

func (t *planningTravels) FindTravelsByTrip(context.Context, uuid.UUID) ([]model.Travel, error) {
	return t.stored, nil
}

func (t *planningTravels) UpsertTravels(_ context.Context, _ uuid.UUID, rows []model.Travel) error {
	t.pending = append(t.pending, rows...)
	return nil
}

func (t *planningTravels) FrozenTravels(context.Context, uuid.UUID) error {
	return repository.ErrInternalServerError
}
