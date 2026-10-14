package run

import (
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"

	"Road-To-Destination-BE/module/realtime"
	triprepo "Road-To-Destination-BE/module/trip/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// activeTripMemberAuthorizer treats the room id as a trip UUID and allows
// only an active member. Another room kind needs its own authorizer.
type activeTripMemberAuthorizer struct {
	db *gorm.DB
}

func (a activeTripMemberAuthorizer) AuthorizeRoom(ctx context.Context, userID uuid.UUID, roomID string) error {
	tripID, err := uuid.Parse(roomID)
	if err != nil {
		return realtime.ErrNotMember
	}
	trip, errTrip := triprepo.NewTripRepository(a.db).FindTripByID(ctx, tripID)
	if errTrip != nil {
		return errTrip
	}
	if trip.Status != enum.TripActive {
		return triprepo.ErrTripNotActive
	}
	_, err = triprepo.NewTripMemberRepository(a.db).FindTripActiveMemberRole(ctx, tripID, userID)
	if errors.Is(err, triprepo.ErrUserNotTripMember) {
		return realtime.ErrNotMember
	}
	return err
}
