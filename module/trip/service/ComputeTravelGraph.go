package service

import (
	"context"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// ComputeTravelGraph routes the destination ids in the request and upserts the travels.
// The saved TripBranch rows are not read as the route: a caller can compute a graph
// before those rows exist, or a different graph than the one already stored.
// The trip still has to be planning, because a locked trip keeps the travels it has.
func (s *ComputeTripService) ComputeTravelGraph(ctx context.Context, tripID uuid.UUID, branches [][]uuid.UUID) (*model.TravelGraph, error) {
	if err := validateTravelBranches(branches); err != nil {
		return nil, err
	}
	trip, err := s.graphs.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip == nil {
		return nil, repository.ErrTripNotFound
	}
	if trip.Status != enum.TripPlanning {
		return nil, repository.ErrInvalidTripGraph
	}
	ids := referencedDestinationIDs(branches)
	found, err := s.graphs.FindDestinationsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	graph, err := resolveGraph(branches, found)
	if err != nil {
		return nil, err
	}
	return s.ComputeTrip(ctx, tripID, graph)
}

func validateTravelBranches(branches [][]uuid.UUID) error {
	if len(branches) == 0 {
		return repository.ErrInvalidTripGraph
	}
	for _, branch := range branches {
		if len(branch) < 2 {
			return repository.ErrInvalidTripGraph
		}
		seen := make(map[uuid.UUID]bool, len(branch))
		for _, id := range branch {
			if id == uuid.Nil || seen[id] {
				return repository.ErrInvalidTripGraph
			}
			seen[id] = true
		}
	}
	return nil
}
