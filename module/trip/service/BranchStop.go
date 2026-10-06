package service

import (
	"context"
	"sort"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// DeleteBranchStop removes one destination from a branch and rewrites that branch's
// stops. Split and merge are derived again from the branch's new ends. The branch
// id stays. A result that would disconnect the graph is rejected and nothing is written.
func (s *TripBranchService) DeleteBranchStop(ctx context.Context, tripID, branchID, destinationID uuid.UUID, myRole enum.TripRole) (*response.TripDetailResponse, error) {
	stored, err := s.loadPlanningGraph(ctx, tripID)
	if err != nil {
		return nil, err
	}
	idx := branchIndex(stored.Branches, branchID)
	if idx < 0 {
		return nil, repository.ErrBranchNotFound
	}
	// Draft is an inbox. Removing a pin from it is DeleteDestination, not a route edit.
	if !stored.Branches[idx].OnRoute() {
		return nil, repository.ErrDraftBranchExcluded
	}
	at := stopIndex(stored.Branches[idx].Stops, destinationID)
	if at < 0 {
		return nil, repository.ErrBranchStopNotFound
	}
	before := snapshotLayouts(stored.Branches)
	stops := stored.Branches[idx].Stops
	next := make([]model.BranchDestination, 0, len(stops)-1)
	next = append(next, stops[:at]...)
	next = append(next, stops[at+1:]...)
	for i := range next {
		next[i].OrderInBranch = i
	}
	stored.Branches[idx].Stops = next
	if err := s.saveMutatedGraph(ctx, stored, before); err != nil {
		return nil, err
	}
	return s.reloadGraph(ctx, tripID, myRole)
}

// UpdateBranchStop replaces the destination at one stop, moves it within the branch, or both.
// At least one field is required. The same graph rules as a full replace apply afterwards.
//
// Steps:
//  1. Reject an empty body — with no field there is nothing to write.
//  2. Only a planning trip can be edited. Stops are ordered by OrderInBranch before the index is taken.
//  3. Edit a copy. The snapshot still reads the stored layout, so the later compare knows which branch actually changed.
//  4. destinationId is the replacement pin and must exist. orderInBranch is the new index in the same branch (0 is the first stop).
//  5. saveMutatedGraph recomputes split and merge. A disconnected graph is not written. Only a branch whose layout changed is stored, and its id stays.
//  6. Reload the stored graph so the response has the saved order and pins.
func (s *TripBranchService) UpdateBranchStop(ctx context.Context, tripID, branchID, destinationID uuid.UUID, req request.UpdateBranchStopRequest, myRole enum.TripRole) (*response.TripDetailResponse, error) {
	// 1. The body carried neither destinationId nor orderInBranch.
	if req.DestinationID == nil && req.OrderInBranch == nil {
		return nil, repository.ErrNoBranchStopUpdate
	}
	// 2. A locked trip keeps its graph. Sort stops first so the index below matches travel order.
	stored, err := s.loadPlanningGraph(ctx, tripID)
	if err != nil {
		return nil, err
	}
	idx := branchIndex(stored.Branches, branchID)
	if idx < 0 {
		return nil, repository.ErrBranchNotFound
	}
	// Draft is an inbox. Reordering it would run split/merge rules that do not apply to loose pins.
	if !stored.Branches[idx].OnRoute() {
		return nil, repository.ErrDraftBranchExcluded
	}
	at := stopIndex(stored.Branches[idx].Stops, destinationID)
	if at < 0 {
		return nil, repository.ErrBranchStopNotFound
	}
	// 3. Copy so the pin and order can change without touching the stored layout. Step 5 snapshots that layout.
	stops := append([]model.BranchDestination(nil), stored.Branches[idx].Stops...)
	if req.DestinationID != nil {
		// 4a. A nil uuid is not a pin. The replacement must already be a destinations row.
		if *req.DestinationID == uuid.Nil {
			return nil, repository.ErrInvalidTripGraph
		}
		found, err := s.branches.FindDestinationsByIDs(ctx, []uuid.UUID{*req.DestinationID})
		if err != nil {
			return nil, err
		}
		if len(found) != 1 {
			return nil, repository.ErrDestinationNotFound
		}
		copied := found[0]
		stops[at].DestinationID = copied.ID
		stops[at].Destination = &copied
	}
	if req.OrderInBranch != nil {
		// 4b. The new index has to stay inside this branch. Past either end, the graph is invalid.
		to := *req.OrderInBranch
		if to < 0 || to >= len(stops) {
			return nil, repository.ErrInvalidTripGraph
		}
		stops = reorderStop(stops, at, to)
	}
	// 5. Snapshot the old layout, then attach the new stops. BuildTripBranches recomputes both ends;
	//    a disconnected sub branch returns before UpdateBranchStops runs.
	before := snapshotLayouts(stored.Branches)
	stored.Branches[idx].Stops = stops
	if err := s.saveMutatedGraph(ctx, stored, before); err != nil {
		return nil, err
	}
	// 6. The row just written has no Destination preload, so reload for the name and coordinates.
	return s.reloadGraph(ctx, tripID, myRole)
}

func (s *TripBranchService) loadPlanningGraph(ctx context.Context, tripID uuid.UUID) (*model.Trip, error) {
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
	sortBranchStops(stored.Branches)
	return stored, nil
}

func sortBranchStops(branches []model.TripBranch) {
	for i := range branches {
		sort.SliceStable(branches[i].Stops, func(a, b int) bool {
			return branches[i].Stops[a].OrderInBranch < branches[i].Stops[b].OrderInBranch
		})
	}
}

func (s *TripBranchService) saveMutatedGraph(ctx context.Context, trip *model.Trip, before map[uuid.UUID]branchLayout) error {
	writes, err := s.pendingLayouts(trip, before)
	if err != nil {
		return err
	}
	for _, branch := range writes {
		if err := s.branches.UpdateBranchStops(ctx, branch); err != nil {
			return err
		}
	}
	return nil
}

// pendingLayouts checks the route after the in-memory stop change.
// A branch whose split, merge, and destination list are unchanged is left out,
// so the write does not recreate stops on a branch the caller did not edit.
// The draft inbox is dropped before that check. It has no split or merge.
func (s *TripBranchService) pendingLayouts(trip *model.Trip, before map[uuid.UUID]branchLayout) ([]model.TripBranch, error) {
	ordered, err := mainFirst(trip.Branches)
	if err != nil {
		return nil, err
	}
	graph, openTail, err := graphFromOrdered(ordered)
	if err != nil {
		return nil, err
	}
	built, err := model.BuildTripBranches(trip, graph, openTail)
	if err != nil {
		return nil, err
	}
	if len(built) != len(ordered) {
		return nil, repository.ErrInvalidTripGraph
	}
	writes := make([]model.TripBranch, 0, len(ordered))
	for i := range ordered {
		next := layoutFromBuilt(ordered[i], built[i])
		prior, ok := before[next.ID]
		if ok && !layoutChanged(prior, next) {
			continue
		}
		writes = append(writes, next)
	}
	return writes, nil
}

type branchLayout struct {
	split *uuid.UUID
	merge *uuid.UUID
	stops []uuid.UUID
}

func snapshotLayouts(branches []model.TripBranch) map[uuid.UUID]branchLayout {
	out := make(map[uuid.UUID]branchLayout, len(branches))
	for i := range branches {
		stops := make([]uuid.UUID, len(branches[i].Stops))
		for j := range branches[i].Stops {
			stops[j] = branches[i].Stops[j].DestinationID
		}
		out[branches[i].ID] = branchLayout{
			split: cloneUUID(branches[i].SplitFromDestinationID),
			merge: cloneUUID(branches[i].MergeToDestinationID),
			stops: stops,
		}
	}
	return out
}

func mainFirst(branches []model.TripBranch) ([]model.TripBranch, error) {
	// A draft branch has no split, so leaving it in this slice looks like a second main branch.
	branches = routeBranches(branches)
	if len(branches) == 0 {
		return nil, repository.ErrInvalidTripGraph
	}
	var main *model.TripBranch
	rest := make([]model.TripBranch, 0, len(branches))
	for i := range branches {
		if branches[i].SplitFromDestinationID == nil {
			if main != nil {
				return nil, repository.ErrInvalidTripGraph
			}
			copied := branches[i]
			main = &copied
			continue
		}
		rest = append(rest, branches[i])
	}
	if main == nil {
		return nil, repository.ErrInvalidTripGraph
	}
	return append([]model.TripBranch{*main}, rest...), nil
}

func routeBranches(branches []model.TripBranch) []model.TripBranch {
	route := make([]model.TripBranch, 0, len(branches))
	for i := range branches {
		if branches[i].OnRoute() {
			route = append(route, branches[i])
		}
	}
	return route
}

func graphFromOrdered(branches []model.TripBranch) (model.BranchGraph, []bool, error) {
	graph := make(model.BranchGraph, len(branches))
	openTail := make([]bool, len(branches))
	for i := range branches {
		stops := branches[i].Stops
		if len(stops) == 0 {
			return nil, nil, repository.ErrInvalidTripGraph
		}
		row := make([]model.Destination, len(stops))
		for j := range stops {
			if stops[j].Destination == nil {
				return nil, nil, model.ErrNilDestination
			}
			row[j] = *stops[j].Destination
		}
		graph[i] = row
		openTail[i] = i > 0 && branches[i].MergeToDestinationID == nil
	}
	return graph, openTail, nil
}

func layoutFromBuilt(existing, built model.TripBranch) model.TripBranch {
	existing.SplitFromDestinationID = cloneUUID(built.SplitFromDestinationID)
	existing.MergeToDestinationID = cloneUUID(built.MergeToDestinationID)
	existing.SplitFrom = nil
	existing.MergeTo = nil
	stops := make([]model.BranchDestination, len(built.Stops))
	for i := range built.Stops {
		stops[i] = model.BranchDestination{
			TripBranchID:  existing.ID,
			DestinationID: built.Stops[i].DestinationID,
			OrderInBranch: i,
		}
	}
	existing.Stops = stops
	return existing
}

func layoutChanged(before branchLayout, next model.TripBranch) bool {
	if !sameUUIDPtr(before.split, next.SplitFromDestinationID) || !sameUUIDPtr(before.merge, next.MergeToDestinationID) {
		return true
	}
	if len(before.stops) != len(next.Stops) {
		return true
	}
	for i := range next.Stops {
		if before.stops[i] != next.Stops[i].DestinationID {
			return true
		}
	}
	return false
}

func reorderStop(stops []model.BranchDestination, from, to int) []model.BranchDestination {
	item := stops[from]
	next := make([]model.BranchDestination, 0, len(stops))
	for i, stop := range stops {
		if len(next) == to {
			next = append(next, item)
		}
		if i == from {
			continue
		}
		next = append(next, stop)
	}
	if len(next) == to {
		next = append(next, item)
	}
	for i := range next {
		next[i].OrderInBranch = i
	}
	return next
}

func branchIndex(branches []model.TripBranch, id uuid.UUID) int {
	for i := range branches {
		if branches[i].ID == id {
			return i
		}
	}
	return -1
}

func stopIndex(stops []model.BranchDestination, destinationID uuid.UUID) int {
	for i := range stops {
		if stops[i].DestinationID == destinationID {
			return i
		}
	}
	return -1
}

func cloneUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil {
		return nil
	}
	copied := *id
	return &copied
}

func sameUUIDPtr(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
