package controller

import (
	"context"

	mapsrepo "Road-To-Destination-BE/module/maps/repository"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
)

// previewLegTTLSeconds is how long a member preview stays in Redis.
// The durable leg cache keeps its own 24 hour window; this one is only a look-ahead.
const previewLegTTLSeconds = 900

// shortTTLLegStore forces a short Redis expiry on every write.
// Find is unchanged, and Route only writes on a miss, so a live key keeps its TTL.
type shortTTLLegStore struct {
	inner mapsrepo.LegStore
}

func (s shortTTLLegStore) Find(ctx context.Context, fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) (*tripmodel.Leg, error) {
	if s.inner == nil {
		return nil, nil
	}
	return s.inner.Find(ctx, fromLat, fromLng, toLat, toLng, vehicle)
}

func (s shortTTLLegStore) Upsert(ctx context.Context, leg *tripmodel.Leg) error {
	if s.inner == nil || leg == nil {
		return nil
	}
	leg.TTLSeconds = previewLegTTLSeconds
	return s.inner.Upsert(ctx, leg)
}

// noopLegStore computes without remembering anything. Used when Redis is not configured.
type noopLegStore struct{}

func (noopLegStore) Find(context.Context, float64, float64, float64, float64, enum.Vehicle) (*tripmodel.Leg, error) {
	return nil, nil
}

func (noopLegStore) Upsert(context.Context, *tripmodel.Leg) error {
	return nil
}

var (
	_ mapsrepo.LegStore = shortTTLLegStore{}
	_ mapsrepo.LegStore = noopLegStore{}
)
