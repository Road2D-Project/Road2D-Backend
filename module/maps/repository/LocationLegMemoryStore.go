package repository

import (
	"context"
	"sync"

	"Road-To-Destination-BE/module/maps/model"
)

type LocationLegMemoryStore struct {
	mu    sync.RWMutex
	byKey map[string]model.LocationLeg
}

func NewLocationLegMemoryStore() *LocationLegMemoryStore {
	return &LocationLegMemoryStore{byKey: make(map[string]model.LocationLeg)}
}

func LegCacheKey(origin, destination, vehicle string) string {
	return origin + "|" + destination + "|" + vehicle
}

func (store *LocationLegMemoryStore) Find(_ context.Context, origin, destination, vehicle string) (*model.LocationLeg, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	leg, ok := store.byKey[LegCacheKey(origin, destination, vehicle)]
	if !ok {
		return nil, nil
	}
	copy := leg
	return &copy, nil
}

func (store *LocationLegMemoryStore) Upsert(_ context.Context, leg *model.LocationLeg) error {
	if leg == nil {
		return nil
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	store.byKey[LegCacheKey(leg.Origin, leg.Destination, leg.Vehicle)] = *leg
	return nil
}
