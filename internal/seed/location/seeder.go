package location

import (
	"context"
	"errors"
	"fmt"

	mapRequest "Road-To-Destination-BE/module/maps/model/request"
	mapResponse "Road-To-Destination-BE/module/maps/model/response"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
)

const defaultGeocodeLimit = 10

const (
	StatusCreated        = "created"
	StatusBackfilled     = "created_from_cache"
	StatusAlreadyInDB    = "skipped_db"
	StatusAlreadyCached  = "skipped_cache"
	StatusDuplicateInRun = "skipped_seen"
	StatusEmptyPlaceID   = "skipped_empty_place_id"
	StatusUnmappedDetail = "skipped_unmapped_detail"
	StatusEmptyDetail    = "skipped_empty_detail"
)

type Event struct {
	PlaceID string
	Name    string
	Status  string
}

type Report struct {
	Pins           int
	GeocodeResults int
	Created        int
	Backfilled     int
	AlreadyInDB    int
	AlreadyCached  int
	DuplicateInRun int
	SkippedEmpty   int
	UnmappedDetail int
	Events         []Event
}

func (r *Report) add(placeID, name, status string) {
	r.Events = append(r.Events, Event{PlaceID: placeID, Name: name, Status: status})
	switch status {
	case StatusCreated:
		r.Created++
	case StatusBackfilled:
		r.Backfilled++
	case StatusAlreadyInDB:
		r.AlreadyInDB++
	case StatusAlreadyCached:
		r.AlreadyCached++
	case StatusDuplicateInRun:
		r.DuplicateInRun++
	case StatusEmptyPlaceID, StatusEmptyDetail:
		r.SkippedEmpty++
	case StatusUnmappedDetail:
		r.UnmappedDetail++
	}
}

func (r Report) Wrote() int {
	return r.Created + r.Backfilled
}

type MapClient interface {
	Geocode(ctx context.Context, req mapRequest.GeocodeRequest) (*mapResponse.GeocodeResponse, error)
	DetailPlace(ctx context.Context, req mapRequest.DetailPlaceRequest) (*mapResponse.PlaceDetailResponse, error)
}

type Store interface {
	FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error)
	CreateLocation(ctx context.Context, location *model.Location) error
}

type Cache interface {
	FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error)
	SetLocation(ctx context.Context, location *model.Location) error
}

type Seeder struct {
	maps      MapClient
	locations Store
	cache     Cache
	Limit     int
}

func NewSeeder(maps MapClient, locations Store, cache Cache) *Seeder {
	return &Seeder{maps: maps, locations: locations, cache: cache, Limit: defaultGeocodeLimit}
}

func (s *Seeder) SeedFromCoords(ctx context.Context, coords []Coord) (*Report, error) {
	report := &Report{Pins: len(coords)}
	if s.maps == nil {
		return report, errors.New("map client is not configured")
	}
	limit := s.Limit
	if limit <= 0 {
		limit = defaultGeocodeLimit
	}
	seen := make(map[string]struct{})
	for _, coord := range coords {
		geo, err := s.maps.Geocode(ctx, mapRequest.GeocodeRequest{
			LatLng:                          fmt.Sprintf("%g,%g", coord.Lat, coord.Lng),
			Limit:                           limit,
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

func (s *Seeder) seedPlaceID(ctx context.Context, placeID, name string, seen map[string]struct{}, report *Report) error {
	if placeID == "" {
		report.add(placeID, name, StatusEmptyPlaceID)
		return nil
	}
	if _, ok := seen[placeID]; ok {
		report.add(placeID, name, StatusDuplicateInRun)
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
			report.add(placeID, loc.Name, StatusBackfilled)
			return nil
		}
		report.add(placeID, loc.Name, StatusAlreadyCached)
		return nil
	}

	if loc, err := s.locations.FindLocationByPlaceID(ctx, placeID); err == nil {
		seen[placeID] = struct{}{}
		s.rememberPlace(ctx, loc)
		report.add(placeID, loc.Name, StatusAlreadyInDB)
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
		report.add(placeID, name, StatusEmptyDetail)
		return nil
	}
	row := detailToLocation(*detail)
	if row == nil {
		seen[placeID] = struct{}{}
		report.add(placeID, name, StatusUnmappedDetail)
		return nil
	}
	if err := s.locations.CreateLocation(ctx, row); err != nil {
		return fmt.Errorf("insert %s: %w", placeID, err)
	}
	seen[placeID] = struct{}{}
	s.rememberPlace(ctx, row)
	report.add(placeID, row.Name, StatusCreated)
	return nil
}

func (s *Seeder) lookupCachedPlace(ctx context.Context, placeID string) (*model.Location, error) {
	if s.cache == nil {
		return nil, nil
	}
	return s.cache.FindLocationByPlaceID(ctx, placeID)
}

func (s *Seeder) rememberPlace(ctx context.Context, row *model.Location) {
	if s.cache == nil || row == nil {
		return
	}
	_ = s.cache.SetLocation(ctx, row)
}

func (s *Seeder) ensureStoredLocation(ctx context.Context, row *model.Location) (bool, error) {
	if row == nil || row.PlaceID == nil {
		return false, nil
	}
	_, err := s.locations.FindLocationByPlaceID(ctx, *row.PlaceID)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, repository.ErrLocationNotFound) {
		return false, err
	}
	if err := s.locations.CreateLocation(ctx, row); err != nil {
		return false, err
	}
	return true, nil
}
