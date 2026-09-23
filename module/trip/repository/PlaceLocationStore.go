package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/module/trip/model"

	"github.com/redis/go-redis/v9"
)

const defaultPlaceCacheTTLSeconds = 86400

// PlaceLocationStore caches verified locations by Goong place_id so seed and
// nearby coordinates skip a second DetailPlace call.
type PlaceLocationStore struct {
	client *redis.Client
	ttl    time.Duration
}

func NewPlaceLocationStore(client *redis.Client) *PlaceLocationStore {
	if client == nil {
		return nil
	}
	seconds := share.GetEnvIntDefault("PLACE_CACHE_TTL_SECONDS", defaultPlaceCacheTTLSeconds)
	return &PlaceLocationStore{
		client: client,
		ttl:    time.Duration(seconds) * time.Second,
	}
}

func PlaceCacheKey(placeID string) string {
	return "place:id:" + placeID
}

func (s *PlaceLocationStore) enabled() bool {
	return s != nil && s.client != nil
}

func (s *PlaceLocationStore) FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error) {
	if !s.enabled() || placeID == "" {
		return nil, nil
	}
	bytes, err := s.client.Get(ctx, PlaceCacheKey(placeID)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var location model.Location
	if err := json.Unmarshal(bytes, &location); err != nil {
		return nil, err
	}
	return &location, nil
}

func (s *PlaceLocationStore) SetLocation(ctx context.Context, location *model.Location) error {
	if !s.enabled() || location == nil || location.PlaceID == nil || *location.PlaceID == "" {
		return nil
	}
	payload, err := json.Marshal(location)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, PlaceCacheKey(*location.PlaceID), payload, s.ttl).Err()
}
