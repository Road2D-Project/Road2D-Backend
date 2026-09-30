package trip

import (
	"context"
	"fmt"
	"strings"

	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/service"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ForkedPin is one destination created for the seed route.
// ID is the pin written onto the main branch. LocationID is the catalog row it was copied from.
type ForkedPin struct {
	LocationID uuid.UUID `json:"locationId"`
	ID         uuid.UUID `json:"destinationId"`
	Name       string    `json:"name"`
}

// DefaultDestinationName is the pin name written at fork.
// The index is part of the name so two catalog places with the same title stay distinct,
// and so the route order is the same index later.
func DefaultDestinationName(idx int, locationName string) string {
	locationName = strings.TrimSpace(locationName)
	if locationName == "" {
		return fmt.Sprintf("seed-destination-%d", idx)
	}
	return fmt.Sprintf("%d. %s", idx, locationName)
}

// ForkLocations copies catalog places into pins, in the given order.
// An empty id list uses the earliest locations, capped by DestinationLimit.
func ForkLocations(ctx context.Context, svc *service.PlaceService, db *gorm.DB, locationIDs []uuid.UUID) ([]ForkedPin, error) {
	list, err := seedLocations(db, locationIDs)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("no locations to fork; run seeder location first")
	}
	forked := make([]ForkedPin, 0, len(list))
	for idx, loc := range list {
		name := DefaultDestinationName(idx, loc.Name)
		created, err := svc.ForkLocation(ctx, loc.ID, request.ForkLocationRequest{Name: name})
		if err != nil {
			return nil, fmt.Errorf("fork destination %d: %w", idx, err)
		}
		forked = append(forked, ForkedPin{LocationID: loc.ID, ID: created.DestinationId, Name: name})
	}
	return forked, nil
}
