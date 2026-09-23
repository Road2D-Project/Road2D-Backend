package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type stubDestinations struct {
	byID    map[uuid.UUID]*model.Destination
	created *model.Destination
	updated *model.Destination
}

func (s *stubDestinations) FindDestinationById(_ context.Context, destinationId uuid.UUID) (*model.Destination, error) {
	destination, ok := s.byID[destinationId]
	if !ok {
		return nil, repository.ErrDestinationNotFound
	}
	copied := *destination
	return &copied, nil
}

func (s *stubDestinations) UpdateDestination(_ context.Context, destination *model.Destination) error {
	copied := *destination
	s.updated = &copied
	s.byID[destination.ID] = &copied
	return nil
}

func (s *stubDestinations) CreateDestination(_ context.Context, destination *model.Destination) error {
	if destination.ID == uuid.Nil {
		destination.ID = uuid.New()
	}
	copied := *destination
	s.created = &copied
	return nil
}

type stubLocations struct {
	byID map[uuid.UUID]*model.Location
}

func (s *stubLocations) FindLocationById(_ context.Context, locationId uuid.UUID) (*model.Location, error) {
	location, ok := s.byID[locationId]
	if !ok {
		return nil, repository.ErrLocationNotFound
	}
	copied := *location
	return &copied, nil
}

func TestUpdateDestinationMovesUnlinkedPin(t *testing.T) {
	id := uuid.New()
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {
			Base:   utils.Base{ID: id},
			Lat:    10,
			Lng:    20,
			Name:   "draft",
			Status: enum.Editing,
		},
	}}
	svc := NewLocationService(destinations, nil, &stubLocations{})

	updated, err := svc.UpdateDestination(context.Background(), id, request.UpdateDestinationRequest{
		Lat:          11,
		Lng:          21,
		Name:         "moved",
		StayOverTime: 15,
		Status:       enum.Editing,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Lat != 11 || updated.Lng != 21 || updated.Name != "moved" {
		t.Fatalf("got lat=%v lng=%v name=%s", updated.Lat, updated.Lng, updated.Name)
	}
}

func TestUpdateDestinationKeepsCoordinatesWhenLinked(t *testing.T) {
	id := uuid.New()
	locationID := uuid.New()
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {
			Base:       utils.Base{ID: id},
			LocationID: &locationID,
			Lat:        10,
			Lng:        20,
			Name:       "forked",
			Status:     enum.Editing,
		},
	}}
	svc := NewLocationService(destinations, nil, &stubLocations{})

	updated, err := svc.UpdateDestination(context.Background(), id, request.UpdateDestinationRequest{
		Lat:    99,
		Lng:    99,
		Name:   "renamed",
		Status: enum.Confirm,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Lat != 10 || updated.Lng != 20 {
		t.Fatalf("coordinates changed to lat=%v lng=%v", updated.Lat, updated.Lng)
	}
	if updated.Name != "renamed" || updated.Status != enum.Confirm {
		t.Fatalf("other fields not applied: name=%s status=%s", updated.Name, updated.Status)
	}
}

func TestUpdateDestinationRejectsConfirmedPin(t *testing.T) {
	id := uuid.New()
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {Base: utils.Base{ID: id}, Status: enum.Confirm},
	}}
	svc := NewLocationService(destinations, nil, &stubLocations{})

	_, err := svc.UpdateDestination(context.Background(), id, request.UpdateDestinationRequest{Name: "nope"})
	if !errors.Is(err, repository.ErrDestinationNotEditing) {
		t.Fatalf("got %v", err)
	}
	if destinations.updated != nil {
		t.Fatal("confirmed pin was written")
	}
}

func TestForkLocationCopiesCoordinatesAndLink(t *testing.T) {
	locationID := uuid.New()
	placeID := "goong-1"
	arrive := time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC)
	locations := &stubLocations{byID: map[uuid.UUID]*model.Location{
		locationID: {
			Base:    utils.Base{ID: locationID},
			Name:    "Ben Thanh",
			Lat:     10.77,
			Lng:     106.7,
			PlaceID: &placeID,
		},
	}}
	destinations := &stubDestinations{}
	svc := NewLocationService(destinations, nil, locations)

	created, err := svc.ForkLocation(context.Background(), locationID, request.ForkLocationRequest{
		Name:         "Morning stop",
		ArriveTime:   &arrive,
		StayOverTime: 30,
	})
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if created.DestinationId == uuid.Nil {
		t.Fatal("destination id was empty")
	}
	if created.LocationId == nil || *created.LocationId != locationID {
		t.Fatalf("location link: %+v", created.LocationId)
	}
	if created.PlaceId == nil || *created.PlaceId != placeID {
		t.Fatalf("place id: %+v", created.PlaceId)
	}
	if destinations.created == nil {
		t.Fatal("destination was not created")
	}
	got := destinations.created
	if got.Lat != 10.77 || got.Lng != 106.7 || got.Name != "Morning stop" || got.Status != enum.Editing {
		t.Fatalf("created pin: %+v", got)
	}
	if got.StayOverTime != 30 || got.ArriveTime == nil || !got.ArriveTime.Equal(arrive) {
		t.Fatalf("schedule: stay=%d arrive=%v", got.StayOverTime, got.ArriveTime)
	}
	if got.CoordinatesEditable() {
		t.Fatal("forked pin should keep the location coordinates")
	}
}

func TestForkLocationMissingPlace(t *testing.T) {
	svc := NewLocationService(&stubDestinations{}, nil, &stubLocations{byID: map[uuid.UUID]*model.Location{}})
	_, err := svc.ForkLocation(context.Background(), uuid.New(), request.ForkLocationRequest{})
	if !errors.Is(err, repository.ErrLocationNotFound) {
		t.Fatalf("got %v", err)
	}
}
