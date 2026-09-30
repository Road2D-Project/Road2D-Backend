package service

import (
	"time"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

// initialMainBranch turns a reviewed location route into pins and hops.
// Location names and coordinates come from the catalog, not from the client payload.
// A nil preview means the trip opens with only its draft inbox.
func (s *TripService) initialMainBranch(ctx context.Context, preview *response.PreviewLocationsResponse) (*model.InitialMainBranch, error) {
	if preview == nil {
		return nil, nil
	}
	if s.locations == nil {
		return nil, repository.ErrLocationNotFound
	}
	stops := preview.Locations
	legs := preview.Legs
	if len(stops) < 2 || len(legs) != len(stops)-1 {
		return nil, repository.ErrInvalidTripGraph
	}
	destinations := make([]model.Destination, len(stops))
	for i, stop := range stops {
		if stop.LocationID == uuid.Nil {
			return nil, repository.ErrInvalidTripGraph
		}
		location, err := s.locations.FindLocationById(ctx, stop.LocationID)
		if err != nil {
			return nil, err
		}
		if location == nil {
			return nil, repository.ErrLocationNotFound
		}
		pin := ForkLocationToDestination(location)
		pin.Location = nil
		pin.ID = uuid.New()
		destinations[i] = *pin
	}
	now := time.Now().UTC()
	travels := make([]model.Travel, len(legs))
	for i, leg := range legs {
		if !legLinks(leg, stops[i].LocationID, stops[i+1].LocationID) {
			return nil, repository.ErrInvalidTripGraph
		}
		travels[i] = model.Travel{
			FromDestinationID: destinations[i].ID,
			ToDestinationID:   destinations[i+1].ID,
			Vehicle:           enum.BIKE,
			Polyline:          leg.Polyline,
			DistanceM:         leg.DistanceM,
			DurationS:         leg.DurationS,
			LastComputedAt:    now,
		}
	}
	return &model.InitialMainBranch{Destinations: destinations, Travels: travels}, nil
}

func legLinks(leg response.ComputeBranchLeg, fromID, toID uuid.UUID) bool {
	if leg.Vehicle != enum.BIKE || leg.Polyline == "" {
		return false
	}
	if leg.From.LocationID == nil || leg.To.LocationID == nil {
		return false
	}
	return *leg.From.LocationID == fromID && *leg.To.LocationID == toID
}
