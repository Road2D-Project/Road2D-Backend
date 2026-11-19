package service

import (
	"context"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// ListDraftDestinations returns the pins parked on the trip's hidden inbox.
// The route graph and stored travels do not include this branch.
func (s *TripBranchService) ListDraftDestinations(ctx context.Context, tripID uuid.UUID) (*response.DraftDestinationsResponse, error) {
	stored, err := s.branches.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, repository.ErrTripNotFound
	}
	draft := findDraftBranch(stored.Branches)
	if draft == nil {
		return &response.DraftDestinationsResponse{Destinations: []response.StopResponse{}}, nil
	}
	sortBranchStops(stored.Branches)
	listed := response.DraftDestinations(draft)
	return &listed, nil
}

// AddDraftDestination appends one existing pin to the trip's draft inbox.
//
// Steps:
//  1. Reject a nil id.
//  2. Only a planning trip accepts new inbox pins.
//  3. The pin must already exist. Fork creates it; this call only parks it.
//  4. One copy per draft branch. A second insert would break idx_branch_dest.
//  5. Append and rewrite that branch only. Split and merge stay empty, and the route graph is not validated.
func (s *TripBranchService) AddDraftDestination(ctx context.Context, tripID, destinationID uuid.UUID) (*response.DraftDestinationsResponse, error) {
	if destinationID == uuid.Nil {
		return nil, repository.ErrDestinationNotFound
	}
	stored, err := s.branches.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, repository.ErrTripNotFound
	}
	if stored.Status != enum.TripPlanning {
		return nil, repository.ErrInvalidTripGraph
	}
	draft := findDraftBranch(stored.Branches)
	if draft == nil {
		return nil, repository.ErrBranchNotFound
	}
	found, err := s.branches.FindDestinationsByIDs(ctx, []uuid.UUID{destinationID})
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, repository.ErrDestinationNotFound
	}
	sortBranchStops(stored.Branches)
	for _, stop := range draft.Stops {
		if stop.DestinationID == destinationID {
			return nil, repository.ErrInvalidTripGraph
		}
	}
	stops := make([]model.BranchDestination, len(draft.Stops)+1)
	copy(stops, draft.Stops)
	for i := range stops {
		stops[i].OrderInBranch = i
		stops[i].Destination = nil
		stops[i].TripBranch = nil
	}
	stops[len(stops)-1] = model.BranchDestination{
		TripBranchID:  draft.ID,
		DestinationID: destinationID,
		OrderInBranch: len(stops) - 1,
	}
	draft.Stops = stops
	draft.TripID = stored.ID
	draft.SplitFrom = nil
	draft.MergeTo = nil
	draft.SplitFromDestinationID = nil
	draft.MergeToDestinationID = nil
	if err := s.branches.UpdateBranchStops(ctx, *draft); err != nil {
		return nil, err
	}
	return s.ListDraftDestinations(ctx, tripID)
}

func findDraftBranch(branches []model.TripBranch) *model.TripBranch {
	for i := range branches {
		if !branches[i].OnRoute() {
			return &branches[i]
		}
	}
	return nil
}
