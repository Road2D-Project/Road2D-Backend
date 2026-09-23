package main

import (
	"context"
	"errors"
	"fmt"

	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
)

const seedGeocodeLimit = 10

const (
	seedStatusCreated        = "created"
	seedStatusBackfilled     = "created_from_cache"
	seedStatusAlreadyInDB    = "skipped_db"
	seedStatusAlreadyCached  = "skipped_cache"
	seedStatusDuplicateInRun = "skipped_seen"
	seedStatusEmptyPlaceID   = "skipped_empty_place_id"
	seedStatusUnmappedDetail = "skipped_unmapped_detail"
	seedStatusEmptyDetail    = "skipped_empty_detail"
)

type seedEvent struct {
	PlaceID string
	Name    string
	Status  string
}

type seedReport struct {
	Pins           int
	GeocodeResults int
	Created        int
	Backfilled     int
	AlreadyInDB    int
	AlreadyCached  int
	DuplicateInRun int
	SkippedEmpty   int
	UnmappedDetail int
	Events         []seedEvent
}

func (r *seedReport) add(placeID, name, status string) {
	r.Events = append(r.Events, seedEvent{PlaceID: placeID, Name: name, Status: status})
	switch status {
	case seedStatusCreated:
		r.Created++
	case seedStatusBackfilled:
		r.Backfilled++
	case seedStatusAlreadyInDB:
		r.AlreadyInDB++
	case seedStatusAlreadyCached:
		r.AlreadyCached++
	case seedStatusDuplicateInRun:
		r.DuplicateInRun++
	case seedStatusEmptyPlaceID, seedStatusEmptyDetail:
		r.SkippedEmpty++
	case seedStatusUnmappedDetail:
		r.UnmappedDetail++
	}
}

func (r seedReport) Wrote() int {
	return r.Created + r.Backfilled
}

type seedMapClient interface {
	Geocode(ctx context.Context, req mapRequest.GeocodeRequest) (*mapResponse.GeocodeResponse, error)
	DetailPlace(ctx context.Context, req mapRequest.DetailPlaceRequest) (*mapResponse.PlaceDetailResponse, error)
}

type seedLocationStore interface {
	FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error)
	CreateLocation(ctx context.Context, location *model.Location) error
}

type seedPlaceCache interface {
	FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error)
	SetLocation(ctx context.Context, location *model.Location) error
}

type locationSeeder struct {
	maps      seedMapClient
	locations seedLocationStore
	cache     seedPlaceCache
}

func newLocationSeeder(maps seedMapClient, locations seedLocationStore, cache seedPlaceCache) *locationSeeder {
	return &locationSeeder{maps: maps, locations: locations, cache: cache}
}

func (s *locationSeeder) seedFromCoords(ctx context.Context, coords []seedCoord) (*seedReport, error) {
	report := &seedReport{Pins: len(coords)}
	if s.maps == nil {
		return report, errors.New("map client is not configured")
	}
	seen := make(map[string]struct{})
	for _, coord := range coords {
		geo, err := s.maps.Geocode(ctx, mapRequest.GeocodeRequest{
			LatLng:                          fmt.Sprintf("%g,%g", coord.Lat, coord.Lng),
			Limit:                           seedGeocodeLimit,
			HasDeprecatedAdministrativeUnit: true,
			HasVNID:                         true,
		})
		if err != nil {
			return report, fmt.Errorf("geocode %g,%g: %w", coord.Lat, coord.Lng, err)
		}
		if geo == nil {
			continue
		}
		report.GeocodeResults += len(geo.Results)
		for _, result := range geo.Results {
			if err := s.seedPlaceID(ctx, result.PlaceID, result.Name, seen, report); err != nil {
				return report, err
			}
		}
	}
	return report, nil
}

func (s *locationSeeder) seedPlaceID(ctx context.Context, placeID, name string, seen map[string]struct{}, report *seedReport) error {
	if placeID == "" {
		report.add(placeID, name, seedStatusEmptyPlaceID)
		return nil
	}
	if _, ok := seen[placeID]; ok {
		report.add(placeID, name, seedStatusDuplicateInRun)
		return nil
	}
	if loc, err := s.lookupCachedPlace(ctx, placeID); err != nil {
		return err
	} else if loc != nil {
		seen[placeID] = struct{}{}
		created, err := s.ensureStoredLocation(ctx, loc)
		if err != nil {
			return err
		}
		if created {
			report.add(placeID, loc.Name, seedStatusBackfilled)
			return nil
		}
		report.add(placeID, loc.Name, seedStatusAlreadyCached)
		return nil
	}

	if loc, err := s.locations.FindLocationByPlaceID(ctx, placeID); err == nil {
		seen[placeID] = struct{}{}
		s.rememberPlace(ctx, loc)
		report.add(placeID, loc.Name, seedStatusAlreadyInDB)
		return nil
	} else if !errors.Is(err, repository.ErrLocationNotFound) {
		return err
	}

	detail, err := s.maps.DetailPlace(ctx, mapRequest.DetailPlaceRequest{
		PlaceId:                         placeID,
		HasDeprecatedAdministrativeUnit: true,
	})
	if err != nil {
		return fmt.Errorf("detail %s: %w", placeID, err)
	}
	if detail == nil {
		seen[placeID] = struct{}{}
		report.add(placeID, name, seedStatusEmptyDetail)
		return nil
	}
	location := detailToLocation(*detail)
	if location == nil {
		seen[placeID] = struct{}{}
		report.add(placeID, name, seedStatusUnmappedDetail)
		return nil
	}
	if err := s.locations.CreateLocation(ctx, location); err != nil {
		return fmt.Errorf("insert %s: %w", placeID, err)
	}
	seen[placeID] = struct{}{}
	s.rememberPlace(ctx, location)
	report.add(placeID, location.Name, seedStatusCreated)
	return nil
}

func (s *locationSeeder) lookupCachedPlace(ctx context.Context, placeID string) (*model.Location, error) {
	if s.cache == nil {
		return nil, nil
	}
	return s.cache.FindLocationByPlaceID(ctx, placeID)
}

func (s *locationSeeder) rememberPlace(ctx context.Context, location *model.Location) {
	if s.cache == nil || location == nil {
		return
	}
	_ = s.cache.SetLocation(ctx, location)
}

func (s *locationSeeder) ensureStoredLocation(ctx context.Context, location *model.Location) (bool, error) {
	if location == nil || location.PlaceID == nil {
		return false, nil
	}
	_, err := s.locations.FindLocationByPlaceID(ctx, *location.PlaceID)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, repository.ErrLocationNotFound) {
		return false, err
	}
	if err := s.locations.CreateLocation(ctx, location); err != nil {
		return false, err
	}
	return true, nil
}
