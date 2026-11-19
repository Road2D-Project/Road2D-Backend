package service

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// PlanningStore is bound to one transaction. The domain services keep their
// graph validation, while this boundary enforces actor permissions and atomicity.
type PlanningStore interface {
	DestinationRowRepository
	LocationRowRepository
	TripBranchRecordRepository
	TripFinderRepository
	TravelStore
	FindTripActiveMemberRole(context.Context, uuid.UUID, uuid.UUID) (enum.TripRole, error)
	DestinationUsages(context.Context, uuid.UUID) ([]repository.DestinationUsage, error)
	InvalidateDestinationTravels(context.Context, uuid.UUID) error
	TouchTrip(context.Context, uuid.UUID) error
}

type PlanningTransaction func(context.Context, func(PlanningStore) error) error

type PlanningService struct {
	transaction   PlanningTransaction
	routes        LegRouter
	previewRoutes LegRouter
	locations     LocationPointFinder
}

func NewPlanningService(transaction PlanningTransaction, routes, previewRoutes LegRouter, locations LocationPointFinder) *PlanningService {
	return &PlanningService{transaction: transaction, routes: routes, previewRoutes: previewRoutes, locations: locations}
}

func canReadDestination(ctx context.Context, store PlanningStore, actorID uuid.UUID, pin *model.Destination) error {
	if actorID == uuid.Nil {
		return repository.ErrDestinationForbidden
	}
	usages, err := store.DestinationUsages(ctx, pin.ID)
	if err != nil {
		return err
	}
	if len(usages) == 0 && pin.CreatedByUserID != nil && *pin.CreatedByUserID == actorID {
		return nil
	}
	for _, usage := range usages {
		if _, err := store.FindTripActiveMemberRole(ctx, usage.TripID, actorID); err == nil {
			return nil
		} else if !errors.Is(err, repository.ErrUserNotTripMember) {
			return err
		}
	}
	return repository.ErrDestinationForbidden
}

func editableDestination(ctx context.Context, store PlanningStore, actorID uuid.UUID, pin *model.Destination) ([]uuid.UUID, error) {
	if err := canReadDestination(ctx, store, actorID, pin); err != nil {
		return nil, err
	}
	if pin.Status != enum.Editing {
		return nil, repository.ErrDestinationNotEditing
	}
	usages, err := store.DestinationUsages(ctx, pin.ID)
	if err != nil {
		return nil, err
	}
	affected := make([]uuid.UUID, 0, len(usages))
	for _, usage := range usages {
		trip, err := store.FindTripByID(ctx, usage.TripID)
		if err != nil {
			return nil, err
		}
		role, err := store.FindTripActiveMemberRole(ctx, usage.TripID, actorID)
		if errors.Is(err, repository.ErrUserNotTripMember) {
			return nil, repository.ErrDestinationForbidden
		}
		if err != nil {
			return nil, err
		}
		ownsDraft := pin.CreatedByUserID != nil && *pin.CreatedByUserID == actorID && !usage.OnRoute && !usage.HasTravel
		if !role.IsLeader() && !ownsDraft {
			return nil, repository.ErrDestinationForbidden
		}
		if trip.Status != enum.TripPlanning || usage.Frozen {
			return nil, repository.ErrDestinationInUse
		}
		affected = append(affected, trip.ID)
	}
	return affected, nil
}

func requirePlanningMember(ctx context.Context, store PlanningStore, actorID, tripID uuid.UUID, leader bool) (*model.Trip, enum.TripRole, error) {
	if actorID == uuid.Nil {
		return nil, 0, repository.ErrUserNotTripMember
	}
	trip, err := store.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, 0, err
	}
	role, err := store.FindTripActiveMemberRole(ctx, tripID, actorID)
	if err != nil {
		return nil, 0, err
	}
	if leader && !role.IsLeader() {
		return nil, 0, repository.ErrDestinationForbidden
	}
	return trip, role, nil
}

func readableDestinations(ctx context.Context, store PlanningStore, actorID uuid.UUID, ids []uuid.UUID) ([]model.Destination, error) {
	pins, err := store.FindDestinationsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	if len(pins) != len(ids) {
		return nil, repository.ErrDestinationNotFound
	}
	for i := range pins {
		if err := canReadDestination(ctx, store, actorID, &pins[i]); err != nil {
			return nil, err
		}
	}
	return pins, nil
}

// Sharing a pin with another trip grants that trip access to it. Reading a
// source trip alone does not authorize extending its audience.
func usableDestinations(ctx context.Context, store PlanningStore, actorID, tripID uuid.UUID, ids []uuid.UUID) ([]model.Destination, error) {
	pins, err := readableDestinations(ctx, store, actorID, ids)
	if err != nil {
		return nil, err
	}
	for i := range pins {
		pin := &pins[i]
		usages, err := store.DestinationUsages(ctx, pin.ID)
		if err != nil {
			return nil, err
		}
		alreadyAttached := false
		for _, usage := range usages {
			alreadyAttached = alreadyAttached || usage.TripID == tripID
		}
		if alreadyAttached {
			continue
		}
		for _, usage := range usages {
			role, err := store.FindTripActiveMemberRole(ctx, usage.TripID, actorID)
			if errors.Is(err, repository.ErrUserNotTripMember) {
				return nil, repository.ErrDestinationForbidden
			}
			if err != nil {
				return nil, err
			}
			ownsDraft := pin.CreatedByUserID != nil && *pin.CreatedByUserID == actorID && !usage.OnRoute && !usage.HasTravel
			if !role.IsLeader() && !ownsDraft {
				return nil, repository.ErrDestinationForbidden
			}
		}
	}
	return pins, nil
}

func (s *PlanningService) GetDestination(ctx context.Context, actorID, destinationID uuid.UUID) (*model.Destination, error) {
	var pin *model.Destination
	err := s.transaction(ctx, func(store PlanningStore) error {
		var err error
		pin, err = store.FindDestinationById(ctx, destinationID)
		if err != nil {
			return err
		}
		return canReadDestination(ctx, store, actorID, pin)
	})
	return pin, err
}

func (s *PlanningService) ForkLocation(ctx context.Context, actorID, locationID uuid.UUID, req request.ForkLocationRequest) (*response.CreateDestinationResponse, error) {
	if actorID == uuid.Nil {
		return nil, repository.ErrDestinationForbidden
	}
	var result *response.CreateDestinationResponse
	err := s.transaction(ctx, func(store PlanningStore) error {
		location, err := store.FindLocationById(ctx, locationID)
		if err != nil {
			return err
		}
		pin := ForkLocationToDestination(location)
		pin.CreatedByUserID = &actorID
		if req.Name != "" {
			pin.Name = req.Name
		}
		pin.StayOverTime = req.StayOverTime
		pin.ArriveTime = req.ArriveTime
		if err := store.CreateDestination(ctx, pin); err != nil {
			return err
		}
		result = &response.CreateDestinationResponse{DestinationId: pin.ID, LocationId: pin.LocationID, PlaceId: location.PlaceID}
		return nil
	})
	return result, err
}

func (s *PlanningService) mutateDestination(ctx context.Context, actorID, id uuid.UUID, mutate func(PlanningStore, *model.Destination) error) ([]uuid.UUID, error) {
	var affected []uuid.UUID
	err := s.transaction(ctx, func(store PlanningStore) error {
		pin, err := store.FindDestinationById(ctx, id)
		if err != nil {
			return err
		}
		affected, err = editableDestination(ctx, store, actorID, pin)
		if err != nil {
			return err
		}
		if err := mutate(store, pin); err != nil {
			return err
		}
		for _, tripID := range affected {
			if err := store.TouchTrip(ctx, tripID); err != nil {
				return err
			}
		}
		return nil
	})
	return affected, err
}

func (s *PlanningService) UpdateDestination(ctx context.Context, actorID, id uuid.UUID, req request.UpdateDestinationRequest) (*model.Destination, []uuid.UUID, error) {
	var updated *model.Destination
	affected, err := s.mutateDestination(ctx, actorID, id, func(store PlanningStore, before *model.Destination) error {
		var err error
		updated, err = NewLocationService(store, store, nil).UpdateDestination(ctx, id, req)
		if err != nil {
			return err
		}
		if before.Lat != updated.Lat || before.Lng != updated.Lng {
			return store.InvalidateDestinationTravels(ctx, id)
		}
		return nil
	})
	return updated, affected, err
}

func (s *PlanningService) PatchDestination(ctx context.Context, actorID, id uuid.UUID, req request.PatchDestinationRequest) (*model.Destination, []uuid.UUID, error) {
	var updated *model.Destination
	affected, err := s.mutateDestination(ctx, actorID, id, func(store PlanningStore, before *model.Destination) error {
		var err error
		updated, err = NewLocationService(store, store, nil).PatchDestination(ctx, id, req)
		if err != nil {
			return err
		}
		if before.Lat != updated.Lat || before.Lng != updated.Lng {
			return store.InvalidateDestinationTravels(ctx, id)
		}
		return nil
	})
	return updated, affected, err
}

func (s *PlanningService) DeleteDestination(ctx context.Context, actorID, id uuid.UUID) ([]uuid.UUID, error) {
	return s.mutateDestination(ctx, actorID, id, func(store PlanningStore, _ *model.Destination) error {
		_, err := NewTripBranchService(store, store).DeleteDestination(ctx, id)
		return err
	})
}

func (s *PlanningService) mutateTrip(ctx context.Context, actorID, tripID uuid.UUID, leader bool, mutate func(PlanningStore, enum.TripRole) error) error {
	return s.transaction(ctx, func(store PlanningStore) error {
		trip, role, err := requirePlanningMember(ctx, store, actorID, tripID, leader)
		if err != nil {
			return err
		}
		if trip.Status != enum.TripPlanning {
			return repository.ErrInvalidTripGraph
		}
		if err := mutate(store, role); err != nil {
			return err
		}
		return store.TouchTrip(ctx, tripID)
	})
}

func (s *PlanningService) AddDraftDestination(ctx context.Context, actorID, tripID, id uuid.UUID) (*response.DraftDestinationsResponse, error) {
	var result *response.DraftDestinationsResponse
	err := s.mutateTrip(ctx, actorID, tripID, false, func(store PlanningStore, _ enum.TripRole) error {
		if _, err := usableDestinations(ctx, store, actorID, tripID, []uuid.UUID{id}); err != nil {
			return err
		}
		var err error
		result, err = NewTripBranchService(store, store).AddDraftDestination(ctx, tripID, id)
		return err
	})
	return result, err
}

func (s *PlanningService) SetTripGraph(ctx context.Context, actorID, tripID uuid.UUID, req request.SetTripGraphRequest) (*response.TripDetailResponse, error) {
	var result *response.TripDetailResponse
	err := s.mutateTrip(ctx, actorID, tripID, true, func(store PlanningStore, role enum.TripRole) error {
		if _, err := usableDestinations(ctx, store, actorID, tripID, referencedDestinationIDs(req.Branches)); err != nil {
			return err
		}
		var err error
		result, err = NewTripBranchService(store, store).SetTripGraph(ctx, tripID, req, role)
		return err
	})
	return result, err
}

func (s *PlanningService) UpdateBranchStop(ctx context.Context, actorID, tripID, branchID, id uuid.UUID, req request.UpdateBranchStopRequest) (*response.TripDetailResponse, error) {
	var result *response.TripDetailResponse
	err := s.mutateTrip(ctx, actorID, tripID, true, func(store PlanningStore, role enum.TripRole) error {
		if req.DestinationID != nil {
			if _, err := usableDestinations(ctx, store, actorID, tripID, []uuid.UUID{*req.DestinationID}); err != nil {
				return err
			}
		}
		var err error
		result, err = NewTripBranchService(store, store).UpdateBranchStop(ctx, tripID, branchID, id, req, role)
		return err
	})
	return result, err
}

func (s *PlanningService) DeleteBranchStop(ctx context.Context, actorID, tripID, branchID, id uuid.UUID) (*response.TripDetailResponse, error) {
	var result *response.TripDetailResponse
	err := s.mutateTrip(ctx, actorID, tripID, true, func(store PlanningStore, role enum.TripRole) error {
		var err error
		result, err = NewTripBranchService(store, store).DeleteBranchStop(ctx, tripID, branchID, id, role)
		return err
	})
	return result, err
}

func (s *PlanningService) UpdateBranch(ctx context.Context, actorID, tripID, branchID uuid.UUID, req request.UpdateBranchRequest) (*response.BranchMetadataResponse, error) {
	var result *response.BranchMetadataResponse
	err := s.transaction(ctx, func(store PlanningStore) error {
		_, role, err := requirePlanningMember(ctx, store, actorID, tripID, false)
		if err != nil {
			return err
		}
		if !role.CanEditTrip() {
			return repository.ErrDestinationForbidden
		}
		result, err = NewTripBranchService(store, store).UpdateBranch(ctx, tripID, branchID, req)
		if err != nil {
			return err
		}
		return store.TouchTrip(ctx, tripID)
	})
	return result, err
}
