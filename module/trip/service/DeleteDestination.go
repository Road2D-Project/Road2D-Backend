package service

import (
	"context"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// DeleteDestination removes an editing pin.
//
// Steps:
//  1. The pin must exist and still be editing.
//  2. Find every trip that still holds it, on the draft inbox or on a route branch.
//  3. Draft only: delete the row. Branch stops cascade. There is no route to validate.
//  4. Any route branch: same rules as editing that graph. A locked trip or a disconnected
//     graph rejects the whole delete. The draft copy is removed when the row goes, by cascade.
//  5. Returned trip ids are the ones whose draft list or route graph changed, so the handler can drop those caches.
func (s *TripBranchService) DeleteDestination(ctx context.Context, destinationID uuid.UUID) ([]uuid.UUID, error) {
	if destinationID == uuid.Nil {
		return nil, repository.ErrDestinationNotFound
	}
	found, err := s.branches.FindDestinationsByIDs(ctx, []uuid.UUID{destinationID})
	if err != nil {
		return nil, err
	}
	if len(found) != 1 {
		return nil, repository.ErrDestinationNotFound
	}
	if found[0].Status != enum.Editing {
		return nil, repository.ErrDestinationNotEditing
	}

	tripIDs, err := s.branches.ListTripIDsByDestination(ctx, destinationID)
	if err != nil {
		return nil, err
	}

	// Classify before any write. A route-graph failure must not remove the pin from another trip.
	var routePlans []destinationRemoval
	affected := make([]uuid.UUID, 0, len(tripIDs))
	for _, tripID := range tripIDs {
		stored, err := s.branches.FindTripWithBranches(ctx, tripID)
		if err != nil {
			return nil, err
		}
		if stored == nil {
			return nil, repository.ErrTripNotFound
		}
		onRoute, onDraft := destinationPlacement(stored, destinationID)
		if !onRoute && !onDraft {
			continue
		}
		affected = append(affected, stored.ID)
		if !onRoute {
			continue
		}
		if stored.Status != enum.TripPlanning {
			return nil, repository.ErrDestinationInUse
		}
		plan, err := s.planDestinationRemoval(ctx, stored.ID, destinationID)
		if err != nil {
			return nil, err
		}
		routePlans = append(routePlans, plan)
	}

	// 3. No route branch holds the pin. Drop the row; draft stops cascade with it.
	if len(routePlans) == 0 {
		if err := s.branches.DeleteDestination(ctx, destinationID); err != nil {
			return nil, err
		}
		return affected, nil
	}

	// 4. Rewrite route branches first. DeleteDestination then cascades any draft stops still pointing here.
	for _, plan := range routePlans {
		for _, branch := range plan.writes {
			if err := s.branches.UpdateBranchStops(ctx, branch); err != nil {
				return nil, err
			}
		}
	}
	if err := s.branches.DeleteDestination(ctx, destinationID); err != nil {
		return nil, err
	}
	return affected, nil
}

type destinationRemoval struct {
	tripID uuid.UUID
	writes []model.TripBranch
}

func (s *TripBranchService) planDestinationRemoval(ctx context.Context, tripID, destinationID uuid.UUID) (destinationRemoval, error) {
	stored, err := s.branches.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return destinationRemoval{}, err
	}
	if stored == nil {
		return destinationRemoval{}, repository.ErrTripNotFound
	}
	if stored.Status != enum.TripPlanning {
		return destinationRemoval{}, repository.ErrDestinationInUse
	}
	sortBranchStops(stored.Branches)
	// Validate the ride only. The draft inbox has no split or merge, and mainFirst ignores it.
	route := routeBranches(stored.Branches)
	before := snapshotLayouts(route)
	routeTrip := *stored
	routeTrip.Branches = route
	stripDestination(&routeTrip, destinationID)
	writes, err := s.pendingLayouts(&routeTrip, before)
	if err != nil {
		return destinationRemoval{}, err
	}
	return destinationRemoval{tripID: stored.ID, writes: writes}, nil
}

func destinationPlacement(trip *model.Trip, destinationID uuid.UUID) (onRoute, onDraft bool) {
	if trip == nil {
		return false, false
	}
	for i := range trip.Branches {
		if stopIndex(trip.Branches[i].Stops, destinationID) < 0 {
			continue
		}
		if trip.Branches[i].OnRoute() {
			onRoute = true
		} else {
			onDraft = true
		}
	}
	return onRoute, onDraft
}

// stripDestination drops the pin from every branch on this trip and compacts OrderInBranch.
// Split and merge are left for pendingLayouts, which recomputes them from the new ends.
func stripDestination(trip *model.Trip, destinationID uuid.UUID) {
	for i := range trip.Branches {
		stops := trip.Branches[i].Stops
		next := make([]model.BranchDestination, 0, len(stops))
		for _, stop := range stops {
			if stop.DestinationID == destinationID {
				continue
			}
			next = append(next, stop)
		}
		for j := range next {
			next[j].OrderInBranch = j
		}
		trip.Branches[i].Stops = next
	}
}
