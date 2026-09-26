package controller

import (
	"context"
	"testing"

	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
)

type recordingLegStore struct {
	found    *tripmodel.Leg
	upserted *tripmodel.Leg
	finds    int
}

func (s *recordingLegStore) Find(context.Context, float64, float64, float64, float64, enum.Vehicle) (*tripmodel.Leg, error) {
	s.finds++
	return s.found, nil
}

func (s *recordingLegStore) Upsert(_ context.Context, leg *tripmodel.Leg) error {
	s.upserted = leg
	return nil
}

func TestShortTTLLegStoreSetsPreviewExpiry(t *testing.T) {
	inner := &recordingLegStore{}
	store := shortTTLLegStore{inner: inner}
	leg := &tripmodel.Leg{Polyline: "preview", TTLSeconds: 86400}

	if err := store.Upsert(context.Background(), leg); err != nil {
		t.Fatal(err)
	}
	if inner.upserted == nil || inner.upserted.TTLSeconds != previewLegTTLSeconds {
		t.Fatalf("ttl = %+v, want %d", inner.upserted, previewLegTTLSeconds)
	}
}

func TestShortTTLLegStoreLeavesFindAlone(t *testing.T) {
	cached := &tripmodel.Leg{Polyline: "cached", TTLSeconds: 86400}
	inner := &recordingLegStore{found: cached}
	store := shortTTLLegStore{inner: inner}

	got, err := store.Find(context.Background(), 1, 2, 3, 4, enum.BIKE)
	if err != nil {
		t.Fatal(err)
	}
	if got != cached || inner.finds != 1 {
		t.Fatalf("find = %+v calls = %d", got, inner.finds)
	}
	if inner.upserted != nil {
		t.Fatal("a read must not rewrite the cached leg")
	}
}

func TestNoopLegStoreRemembersNothing(t *testing.T) {
	var store noopLegStore
	got, err := store.Find(context.Background(), 1, 2, 3, 4, enum.BIKE)
	if err != nil || got != nil {
		t.Fatalf("find = %+v err = %v", got, err)
	}
	if err := store.Upsert(context.Background(), &tripmodel.Leg{}); err != nil {
		t.Fatal(err)
	}
}
