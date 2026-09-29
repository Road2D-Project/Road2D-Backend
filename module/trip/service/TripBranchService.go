package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

type TripBranchRecordRepository interface {
	FindDestinationsByIDs(ctx context.Context, ids []uuid.UUID) ([]model.Destination, error)
	ReplaceTripBranches(ctx context.Context, tripID uuid.UUID, branches []model.TripBranch) error
	FindTripWithBranches(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	UpdateBranchStops(ctx context.Context, branch model.TripBranch) error
	ListTripIDsByDestination(ctx context.Context, destinationID uuid.UUID) ([]uuid.UUID, error)
	DeleteDestination(ctx context.Context, destinationID uuid.UUID) error
}
type TripFinderRepository interface {
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}
type TripBranchService struct {
	branches TripBranchRecordRepository
	trip     TripFinderRepository
}

func NewTripBranchService(repo TripBranchRecordRepository, trip TripFinderRepository) *TripBranchService {
	return &TripBranchService{branches: repo, trip: trip}
}

// SetTripGraph replaces the route graph of a planning trip. The request carries
// only destination ids; they are resolved, checked against the branch rules and
// written back as branches and ordered stops.
func (s *TripBranchService) SetTripGraph(ctx context.Context, tripID uuid.UUID, req request.SetTripGraphRequest, myRole enum.TripRole) (*response.TripDetailResponse, error) {
	trip, err := s.trip.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip.Status != enum.TripPlanning {
		return nil, repository.ErrInvalidTripGraph
	}
	if err := validateGraphShape(req); err != nil {
		return nil, err
	}

	ids := referencedDestinationIDs(req.Branches)
	destinations, err := s.branches.FindDestinationsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	graph, err := resolveGraph(req.Branches, destinations)
	if err != nil {
		return nil, err
	}

	branches, err := model.BuildTripBranches(trip, graph, req.OpenTail)
	if err != nil {
		return nil, err
	}
	if err := s.branches.ReplaceTripBranches(ctx, tripID, branches); err != nil {
		return nil, err
	}
	return s.reloadGraph(ctx, tripID, myRole)
}

// GetTripGraph returns the trip and the branches already stored for it.
// A trip that has not been given a graph yet still returns, with an empty branch list.
// Locked trips stay readable; only writing the graph is limited to planning.
func (s *TripBranchService) GetTripGraph(ctx context.Context, tripID uuid.UUID, myRole enum.TripRole) (*response.TripDetailResponse, error) {
	return s.reloadGraph(ctx, tripID, myRole)
}

func (s *TripBranchService) reloadGraph(ctx context.Context, tripID uuid.UUID, myRole enum.TripRole) (*response.TripDetailResponse, error) {
	stored, err := s.branches.FindTripWithBranches(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, repository.ErrTripNotFound
	}
	detail := response.FromTripDetail(stored, response.RolePtr(myRole))
	return &detail, nil
}

// validateGraphShape checks the rules that do not need the database: the tail
// flags line up with the branches, the main branch is never open ended, every
// sub branch has two ends, and no destination repeats inside one branch.
func validateGraphShape(req request.SetTripGraphRequest) error {
	if len(req.Branches) == 0 || len(req.OpenTail) != len(req.Branches) || req.OpenTail[0] {
		return repository.ErrInvalidTripGraph
	}
	for i, branch := range req.Branches {
		if len(branch) == 0 || (i > 0 && len(branch) < 2) {
			return repository.ErrInvalidTripGraph
		}
		seen := make(map[uuid.UUID]bool, len(branch))
		for _, id := range branch {
			if seen[id] {
				return repository.ErrInvalidTripGraph
			}
			seen[id] = true
		}
	}
	return nil
}

// resolveGraph turns destination ids into the branch rows ComputeTrip and
// BuildTripBranches walk. A missing id is an unknown pin, not an empty stop.
func resolveGraph(branches [][]uuid.UUID, destinations []model.Destination) (model.GraphBranch, error) {
	byID := make(map[uuid.UUID]model.Destination, len(destinations))
	for _, d := range destinations {
		byID[d.ID] = d
	}
	graph := make(model.GraphBranch, len(branches))
	for i, branch := range branches {
		resolved := make([]model.Destination, len(branch))
		for j, id := range branch {
			d, ok := byID[id]
			if !ok {
				return nil, repository.ErrDestinationNotFound
			}
			resolved[j] = d
		}
		graph[i] = resolved
	}
	return graph, nil
}

// referencedDestinationIDs collects each destination id once, in first-seen
// order, so the whole graph resolves in a single query.
func referencedDestinationIDs(branches [][]uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]bool)
	ids := make([]uuid.UUID, 0)
	for _, branch := range branches {
		for _, id := range branch {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	return ids
}
