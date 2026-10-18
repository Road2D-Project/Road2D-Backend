package repository

import (
	"context"
	"log"

	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// legIDStore is the Postgres check behind a Redis hit. Travels store leg_id as
// a foreign key, so a cached id that is not a row has to be ignored.
type legIDStore interface {
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}

// CachedLegStore chains the two leg caches: Redis for speed, Postgres for the
// copy that survives a restart. A miss in both leaves the caller to call Goong.
type CachedLegStore struct {
	memory LegStore
	rows   LegStore
}

func NewCachedLegStore(memory LegStore, rows LegStore) *CachedLegStore {
	return &CachedLegStore{memory: memory, rows: rows}
}

// Find warms Redis from Postgres on a memory miss. A broken Redis only costs a
// database read, so its errors are logged and the lookup continues.
func (s *CachedLegStore) Find(ctx context.Context, fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) (*tripmodel.Leg, error) {
	if s.memory != nil {
		leg, err := s.memory.Find(ctx, fromLat, fromLng, toLat, toLng, vehicle)
		if err != nil {
			log.Printf("leg cache read failed, falling back to database: %v", err)
		} else if leg != nil && !s.staleLegID(ctx, leg.ID) {
			return leg, nil
		}
	}
	if s.rows == nil {
		return nil, nil
	}
	leg, err := s.rows.Find(ctx, fromLat, fromLng, toLat, toLng, vehicle)
	if err != nil || leg == nil {
		return nil, err
	}
	if s.memory != nil {
		if err := s.memory.Upsert(ctx, leg); err != nil {
			log.Printf("leg cache warm failed: %v", err)
		}
	}
	return leg, nil
}

// Upsert writes Postgres first so the leg carries its row id before it is cached.
func (s *CachedLegStore) Upsert(ctx context.Context, leg *tripmodel.Leg) error {
	if s.rows != nil {
		if err := s.rows.Upsert(ctx, leg); err != nil {
			return err
		}
	}
	if s.memory == nil {
		return nil
	}
	if err := s.memory.Upsert(ctx, leg); err != nil {
		log.Printf("leg cache write failed: %v", err)
	}
	return nil
}

// staleLegID is true when Redis still holds an id that Postgres no longer has.
// The caller then reads the coordinate row, or misses and computes a new one.
func (s *CachedLegStore) staleLegID(ctx context.Context, id uuid.UUID) bool {
	if id == uuid.Nil || s.rows == nil {
		return false
	}
	lookup, ok := s.rows.(legIDStore)
	if !ok {
		return false
	}
	found, err := lookup.Exists(ctx, id)
	if err != nil {
		log.Printf("leg id check failed, reading the database row: %v", err)
		return true
	}
	return !found
}

var _ LegStore = (*CachedLegStore)(nil)
