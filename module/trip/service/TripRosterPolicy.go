package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

type TripRosterFinder interface {
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

// requireMutableTripRoster keeps every membership mutation behind one policy.
//
// Future policy may allow explicit join and kick operations while a trip is
// active. Keep that exception narrow: realtime room membership and recovery
// state must be updated atomically before those operations bypass this guard.
func requireMutableTripRoster(trip *model.Trip) error {
	if trip == nil {
		return repository.ErrTripNotFound
	}
	if trip.Status == enum.TripActive {
		return repository.ErrTripRosterLocked
	}
	return nil
}
