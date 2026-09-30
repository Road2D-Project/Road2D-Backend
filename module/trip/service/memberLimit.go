package service

import (
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
)

func memberLimitFor(tripType enum.TripType) (int, error) {
	limit, ok := tripType.MemberLimit()
	if !ok {
		return 0, repository.ErrTripTypePolicyUnset
	}
	return limit, nil
}

// seatsFit rejects a roster that would pass the frozen cap. limit <= 0 means
// the tier never produced a cap, so the write is refused.
func seatsFit(seats, limit int) error {
	if limit <= 0 || seats > limit {
		return repository.ErrTripMemberLimit
	}
	return nil
}
