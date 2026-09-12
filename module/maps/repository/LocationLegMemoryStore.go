package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"Road-To-Destination-BE/module/share"
	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/redis/go-redis/v9"
)

const defaultLegCacheTTLSeconds = 86400

type LegStore interface {
	Find(ctx context.Context, fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) (*tripmodel.Leg, error)
	Upsert(ctx context.Context, leg *tripmodel.Leg) error
}

type LocationLegMemoryStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewLocationLegMemoryStore(client *redis.Client) *LocationLegMemoryStore {
	seconds := share.GetEnvIntDefault("LEG_CACHE_TTL_SECONDS", defaultLegCacheTTLSeconds)
	return &LocationLegMemoryStore{
		client: client,
		ttl:    time.Duration(seconds) * time.Second,
	}
}

func LegCacheKey(fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) string {
	return "leg:" + formatCoord(fromLat) + "," + formatCoord(fromLng) + "|" +
		formatCoord(toLat) + "," + formatCoord(toLng) + "|" + vehicle.Goong()
}

func formatCoord(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (store *LocationLegMemoryStore) Find(ctx context.Context, fromLat, fromLng, toLat, toLng float64, vehicle enum.Vehicle) (*tripmodel.Leg, error) {
	bytes, err := store.client.Get(ctx, LegCacheKey(fromLat, fromLng, toLat, toLng, vehicle)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var leg tripmodel.Leg
	if err := json.Unmarshal(bytes, &leg); err != nil {
		return nil, err
	}
	return &leg, nil
}

func (store *LocationLegMemoryStore) Upsert(ctx context.Context, leg *tripmodel.Leg) error {
	if leg == nil {
		return nil
	}
	payload, err := json.Marshal(leg)
	if err != nil {
		return err
	}
	ttl := store.ttl
	if leg.TTLSeconds > 0 {
		ttl = time.Duration(leg.TTLSeconds) * time.Second
	}
	key := LegCacheKey(leg.FromLat, leg.FromLng, leg.ToLat, leg.ToLng, leg.Vehicle)
	return store.client.Set(ctx, key, payload, ttl).Err()
}

var _ LegStore = (*LocationLegMemoryStore)(nil)
