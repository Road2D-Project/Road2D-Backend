package main

import (
	"context"
	"errors"
	"testing"

	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"

	"github.com/google/uuid"
)

type stubMapClient struct {
	geocode   *mapResponse.GeocodeResponse
	details   map[string]mapResponse.PlaceDetailResponse
	geocodeN  int
	detailIDs []string
}

func (s *stubMapClient) Geocode(context.Context, mapRequest.GeocodeRequest) (*mapResponse.GeocodeResponse, error) {
	s.geocodeN++
	return s.geocode, nil
}

func (s *stubMapClient) DetailPlace(_ context.Context, req mapRequest.DetailPlaceRequest) (*mapResponse.PlaceDetailResponse, error) {
	s.detailIDs = append(s.detailIDs, req.PlaceId)
	out, ok := s.details[req.PlaceId]
	if !ok {
		return nil, errors.New("unknown place_id")
	}
	copied := out
	return &copied, nil
}

type stubLocations struct {
	byPlaceID map[string]*model.Location
	created   []*model.Location
}

func (s *stubLocations) FindLocationByPlaceID(_ context.Context, placeID string) (*model.Location, error) {
	if s.byPlaceID == nil {
		return nil, repository.ErrLocationNotFound
	}
	location, ok := s.byPlaceID[placeID]
	if !ok {
		return nil, repository.ErrLocationNotFound
	}
	copied := *location
	return &copied, nil
}

func (s *stubLocations) CreateLocation(_ context.Context, location *model.Location) error {
	if location.ID == uuid.Nil {
		location.ID = uuid.New()
	}
	copied := *location
	if copied.PlaceID != nil {
		if s.byPlaceID == nil {
			s.byPlaceID = map[string]*model.Location{}
		}
		s.byPlaceID[*copied.PlaceID] = &copied
	}
	s.created = append(s.created, &copied)
	return nil
}

type stubPlaceCache struct {
	byPlaceID map[string]*model.Location
}

func (s *stubPlaceCache) FindLocationByPlaceID(_ context.Context, placeID string) (*model.Location, error) {
	if s == nil || s.byPlaceID == nil {
		return nil, nil
	}
	loc, ok := s.byPlaceID[placeID]
	if !ok {
		return nil, nil
	}
	copied := *loc
	return &copied, nil
}

func (s *stubPlaceCache) SetLocation(_ context.Context, location *model.Location) error {
	if s.byPlaceID == nil {
		s.byPlaceID = map[string]*model.Location{}
	}
	if location == nil || location.PlaceID == nil {
		return nil
	}
	copied := *location
	s.byPlaceID[*location.PlaceID] = &copied
	return nil
}

func sampleDetail(placeID, name string, lat, lng float64) mapResponse.PlaceDetailResponse {
	return mapResponse.PlaceDetailResponse{
		Status: "OK",
		Result: mapResponse.PlaceDetail{
			PlaceID:          placeID,
			Name:             name,
			FormattedAddress: name,
			Geometry:         &mapResponse.PlaceGeometry{Location: mapResponse.LatLng{Lat: lat, Lng: lng}},
			Compound:         &mapResponse.AdministrativeCompound{Commune: "Linh Xuân", Province: "Hồ Chí Minh"},
		},
	}
}

func TestSeedLocationsDetailsEachNewPlaceID(t *testing.T) {
	maps := &stubMapClient{
		geocode: &mapResponse.GeocodeResponse{
			Status: "OK",
			Results: []mapResponse.GeocodeResult{
				{PlaceID: "place-a", Name: "A"},
				{PlaceID: "place-b", Name: "B"},
			},
		},
		details: map[string]mapResponse.PlaceDetailResponse{
			"place-a": sampleDetail("place-a", "A", 10.87, 106.80),
			"place-b": sampleDetail("place-b", "B", 10.88, 106.81),
		},
	}
	locations := &stubLocations{}
	cache := &stubPlaceCache{}
	seeder := newLocationSeeder(maps, locations, cache)

	report, err := seeder.seedFromCoords(context.Background(), []seedCoord{{Lat: 10.8721512, Lng: 106.803008}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if report.Created != 2 {
		t.Fatalf("created=%d events=%+v", report.Created, report.Events)
	}
	if maps.geocodeN != 1 {
		t.Fatalf("geocode calls=%d", maps.geocodeN)
	}
	if len(maps.detailIDs) != 2 {
		t.Fatalf("detail calls=%v", maps.detailIDs)
	}
	if len(locations.created) != 2 {
		t.Fatalf("created=%d", len(locations.created))
	}
	if cache.byPlaceID["place-a"] == nil || cache.byPlaceID["place-b"] == nil {
		t.Fatal("place_ids were not cached")
	}
}

func TestSeedLocationsSkipsCachedPlaceID(t *testing.T) {
	placeID := "place-a"
	maps := &stubMapClient{
		geocode: &mapResponse.GeocodeResponse{
			Status:  "OK",
			Results: []mapResponse.GeocodeResult{{PlaceID: placeID}},
		},
		details: map[string]mapResponse.PlaceDetailResponse{
			placeID: sampleDetail(placeID, "A", 10.87, 106.80),
		},
	}
	loc := detailToLocation(sampleDetail(placeID, "A", 10.87, 106.80))
	loc.ID = utils.Base{}.ID
	cache := &stubPlaceCache{byPlaceID: map[string]*model.Location{placeID: loc}}
	locations := &stubLocations{byPlaceID: map[string]*model.Location{placeID: loc}}
	seeder := newLocationSeeder(maps, locations, cache)

	report, err := seeder.seedFromCoords(context.Background(), []seedCoord{
		{Lat: 10.87, Lng: 106.80},
		{Lat: 10.88, Lng: 106.81},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if report.Wrote() != 0 {
		t.Fatalf("wrote=%d events=%+v", report.Wrote(), report.Events)
	}
	if len(maps.detailIDs) != 0 {
		t.Fatalf("detail should be skipped, got %v", maps.detailIDs)
	}
}

func TestSeedLocationsSkipsDatabasePlaceID(t *testing.T) {
	placeID := "place-db"
	maps := &stubMapClient{
		geocode: &mapResponse.GeocodeResponse{
			Status:  "OK",
			Results: []mapResponse.GeocodeResult{{PlaceID: placeID}, {PlaceID: placeID}},
		},
		details: map[string]mapResponse.PlaceDetailResponse{
			placeID: sampleDetail(placeID, "DB", 1, 2),
		},
	}
	existing := detailToLocation(sampleDetail(placeID, "DB", 1, 2))
	locations := &stubLocations{byPlaceID: map[string]*model.Location{placeID: existing}}
	seeder := newLocationSeeder(maps, locations, &stubPlaceCache{})

	report, err := seeder.seedFromCoords(context.Background(), []seedCoord{{Lat: 10, Lng: 106}})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if report.AlreadyInDB == 0 {
		t.Fatalf("expected skipped_db, events=%+v", report.Events)
	}
	if len(maps.detailIDs) != 0 {
		t.Fatalf("detail should be skipped, got %v", maps.detailIDs)
	}
}

func TestDetailToLocationRequiresGeometry(t *testing.T) {
	if loc := detailToLocation(mapResponse.PlaceDetailResponse{
		Result: mapResponse.PlaceDetail{PlaceID: "x", Name: "x"},
	}); loc != nil {
		t.Fatal("expected nil without geometry")
	}
}
