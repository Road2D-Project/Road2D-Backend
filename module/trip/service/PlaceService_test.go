package service

import (
	"context"
	"errors"
	"testing"
	"time"

	mapsclient "Road-To-Destination-BE/module/maps/client"
	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
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
	byID      map[uuid.UUID]*model.Location
	byPlace   map[string]*model.Location
	created   *model.Location
	createErr error
	findErr   error
	finds     int
}

func (s *stubLocations) FindLocationById(_ context.Context, locationId uuid.UUID) (*model.Location, error) {
	location, ok := s.byID[locationId]
	if !ok {
		return nil, repository.ErrLocationNotFound
	}
	copied := *location
	return &copied, nil
}

func (s *stubLocations) FindLocationByPlaceID(_ context.Context, placeID string) (*model.Location, error) {
	s.finds++
	if s.findErr != nil && s.finds == 1 {
		return nil, s.findErr
	}
	location, ok := s.byPlace[placeID]
	if !ok {
		return nil, repository.ErrLocationNotFound
	}
	copied := *location
	return &copied, nil
}

func (s *stubLocations) CreateLocation(_ context.Context, location *model.Location) error {
	if s.createErr != nil {
		return s.createErr
	}
	if location.ID == uuid.Nil {
		location.ID = uuid.New()
	}
	copied := *location
	s.created = &copied
	if s.byPlace == nil {
		s.byPlace = map[string]*model.Location{}
	}
	if location.PlaceID != nil {
		s.byPlace[*location.PlaceID] = &copied
	}
	return nil
}

type stubPlaceDetails struct {
	detail *mapResponse.PlaceDetailResponse
	err    error
	calls  int
}

func (s *stubPlaceDetails) DetailPlace(context.Context, mapRequest.DetailPlaceRequest) (*mapResponse.PlaceDetailResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.detail, nil
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
	svc := NewLocationService(destinations, &stubLocations{}, nil)

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
	svc := NewLocationService(destinations, &stubLocations{}, nil)

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
	svc := NewLocationService(destinations, &stubLocations{}, nil)

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
	svc := NewLocationService(destinations, locations, nil)

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

func TestPatchDestinationKeepsUnsentFields(t *testing.T) {
	id := uuid.New()
	locationID := uuid.New()
	stay := 40
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {
			Base:         utils.Base{ID: id},
			LocationID:   &locationID,
			Lat:          10,
			Lng:          20,
			Name:         "forked",
			StayOverTime: 5,
			Status:       enum.Editing,
		},
	}}
	svc := NewLocationService(destinations, &stubLocations{}, nil)
	lat := 99.0

	updated, err := svc.PatchDestination(context.Background(), id, request.PatchDestinationRequest{
		StayOverTime: &stay,
		Lat:          &lat,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.StayOverTime != 40 || updated.Name != "forked" || updated.Lat != 10 || updated.Lng != 20 {
		t.Fatalf("got %+v", updated)
	}
}

func TestPatchDestinationMovesUnlinkedPin(t *testing.T) {
	id := uuid.New()
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {Base: utils.Base{ID: id}, Lat: 10, Lng: 20, Name: "draft", Status: enum.Editing},
	}}
	svc := NewLocationService(destinations, &stubLocations{}, nil)
	lat := 11.0
	name := "moved"

	updated, err := svc.PatchDestination(context.Background(), id, request.PatchDestinationRequest{
		Name: &name,
		Lat:  &lat,
	})
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	if updated.Name != "moved" || updated.Lat != 11 || updated.Lng != 20 || updated.StayOverTime != 0 {
		t.Fatalf("got %+v", updated)
	}
}

func TestPatchDestinationRejectsEmptyBody(t *testing.T) {
	id := uuid.New()
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {Base: utils.Base{ID: id}, Status: enum.Editing},
	}}
	svc := NewLocationService(destinations, &stubLocations{}, nil)

	_, err := svc.PatchDestination(context.Background(), id, request.PatchDestinationRequest{})
	if !errors.Is(err, repository.ErrNoDestinationUpdate) {
		t.Fatalf("got %v", err)
	}
	if destinations.updated != nil {
		t.Fatal("empty patch was written")
	}
}

func TestPatchDestinationRejectsConfirmedPin(t *testing.T) {
	id := uuid.New()
	stay := 10
	destinations := &stubDestinations{byID: map[uuid.UUID]*model.Destination{
		id: {Base: utils.Base{ID: id}, Status: enum.Confirm},
	}}
	svc := NewLocationService(destinations, &stubLocations{}, nil)

	_, err := svc.PatchDestination(context.Background(), id, request.PatchDestinationRequest{StayOverTime: &stay})
	if !errors.Is(err, repository.ErrDestinationNotEditing) {
		t.Fatalf("got %v", err)
	}
}

func TestForkLocationMissingPlace(t *testing.T) {
	svc := NewLocationService(&stubDestinations{}, &stubLocations{byID: map[uuid.UUID]*model.Location{}}, nil)
	_, err := svc.ForkLocation(context.Background(), uuid.New(), request.ForkLocationRequest{})
	if !errors.Is(err, repository.ErrLocationNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureLocationReturnsStoredPlace(t *testing.T) {
	locationID := uuid.New()
	placeID := "goong-1"
	locations := &stubLocations{byPlace: map[string]*model.Location{
		placeID: {
			Base:    utils.Base{ID: locationID},
			Name:    "Ben Thanh",
			Lat:     10.77,
			Lng:     106.7,
			PlaceID: &placeID,
		},
	}}
	details := &stubPlaceDetails{}
	svc := NewLocationService(&stubDestinations{}, locations, details)

	got, created, err := svc.EnsureLocation(context.Background(), "  goong-1  ")
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if created {
		t.Fatal("stored place was inserted again")
	}
	if details.calls != 0 {
		t.Fatalf("detail calls: %d", details.calls)
	}
	if got.LocationID != locationID || got.PlaceID != placeID || got.Name != "Ben Thanh" || got.Lat != 10.77 || got.Lng != 106.7 {
		t.Fatalf("got %+v", got)
	}
}

func TestEnsureLocationStoresPlaceDetail(t *testing.T) {
	placeID := "goong-2"
	details := &stubPlaceDetails{detail: &mapResponse.PlaceDetailResponse{
		Result: mapResponse.PlaceDetail{
			PlaceID:          placeID,
			Name:             "Cafe",
			FormattedAddress: "1 Le Loi",
			Geometry:         &mapResponse.PlaceGeometry{Location: mapResponse.LatLng{Lat: 10.5, Lng: 106.6}},
			Compound:         &mapResponse.AdministrativeCompound{Commune: "Ben Thanh", District: "1", Province: "Ho Chi Minh"},
		},
	}}
	locations := &stubLocations{}
	svc := NewLocationService(&stubDestinations{}, locations, details)

	got, created, err := svc.EnsureLocation(context.Background(), placeID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !created || details.calls != 1 || locations.created == nil {
		t.Fatalf("created=%v calls=%d row=%+v", created, details.calls, locations.created)
	}
	if got.LocationID == uuid.Nil || got.PlaceID != placeID || got.Name != "Cafe" || got.Lat != 10.5 || got.Lng != 106.6 {
		t.Fatalf("got %+v", got)
	}
	if !locations.created.IsVerified || locations.created.PlaceID == nil || *locations.created.PlaceID != placeID {
		t.Fatalf("stored row: %+v", locations.created)
	}
}

func TestEnsureLocationRejectsEmptyDetail(t *testing.T) {
	locations := &stubLocations{}
	details := &stubPlaceDetails{detail: &mapResponse.PlaceDetailResponse{
		Result: mapResponse.PlaceDetail{PlaceID: "goong-3"},
	}}
	svc := NewLocationService(&stubDestinations{}, locations, details)

	_, _, err := svc.EnsureLocation(context.Background(), "goong-3")
	if !errors.Is(err, ErrUnusablePlaceDetail) {
		t.Fatalf("got %v", err)
	}
	if locations.created != nil {
		t.Fatal("unusable detail was stored")
	}
}

func TestEnsureLocationRequiresPlaceID(t *testing.T) {
	svc := NewLocationService(&stubDestinations{}, &stubLocations{}, &stubPlaceDetails{})
	_, _, err := svc.EnsureLocation(context.Background(), "  ")
	if !errors.Is(err, ErrPlaceIDRequired) {
		t.Fatalf("got %v", err)
	}
}

func TestEnsureLocationReturnsWinnerAfterInsertRace(t *testing.T) {
	placeID := "goong-4"
	winnerID := uuid.New()
	locations := &stubLocations{
		findErr:   repository.ErrLocationNotFound,
		createErr: errors.New("duplicate place_id"),
		byPlace: map[string]*model.Location{
			placeID: {
				Base:    utils.Base{ID: winnerID},
				Name:    "Winner",
				Lat:     1,
				Lng:     2,
				PlaceID: &placeID,
			},
		},
	}
	svc := NewLocationService(&stubDestinations{}, locations, &stubPlaceDetails{detail: &mapResponse.PlaceDetailResponse{
		Result: mapResponse.PlaceDetail{
			PlaceID:  placeID,
			Name:     "Loser",
			Geometry: &mapResponse.PlaceGeometry{Location: mapResponse.LatLng{Lat: 9, Lng: 9}},
		},
	}})

	got, created, err := svc.EnsureLocation(context.Background(), placeID)
	if err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if created || got.LocationID != winnerID || got.Name != "Winner" {
		t.Fatalf("created=%v got %+v", created, got)
	}
}

func TestEnsureLocationNeedsADetailClient(t *testing.T) {
	svc := NewLocationService(&stubDestinations{}, &stubLocations{}, nil)
	_, _, err := svc.EnsureLocation(context.Background(), "goong-5")
	if !errors.Is(err, mapsclient.ErrMissingAPIKey) {
		t.Fatalf("got %v", err)
	}
}
