package repository

import (
	"context"
	"errors"
	"testing"

	tripmodel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

type stubLegStore struct {
	leg       *tripmodel.Leg
	findErr   error
	upsertErr error
	findCalls int
	upserted  []tripmodel.Leg
	// assignID mimics the Postgres RETURNING that stamps a row id on the leg.
	assignID uuid.UUID
}

func (s *stubLegStore) Find(_ context.Context, _, _, _, _ float64, _ enum.Vehicle) (*tripmodel.Leg, error) {
	s.findCalls++
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.leg, nil
}

func (s *stubLegStore) Upsert(_ context.Context, leg *tripmodel.Leg) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	if s.assignID != uuid.Nil {
		leg.ID = s.assignID
	}
	s.upserted = append(s.upserted, *leg)
	return nil
}

func find(t *testing.T, store *CachedLegStore) *tripmodel.Leg {
	t.Helper()
	leg, err := store.Find(context.Background(), 1, 2, 3, 4, enum.BIKE)
	if err != nil {
		t.Fatal(err)
	}
	return leg
}

func TestCachedLegStoreMemoryHitSkipsDatabase(t *testing.T) {
	memory := &stubLegStore{leg: &tripmodel.Leg{Polyline: "from-redis"}}
	rows := &stubLegStore{leg: &tripmodel.Leg{Polyline: "from-postgres"}}

	leg := find(t, NewCachedLegStore(memory, rows))

	if leg == nil || leg.Polyline != "from-redis" {
		t.Fatalf("leg = %+v", leg)
	}
	if rows.findCalls != 0 {
		t.Fatal("a memory hit must not touch the database")
	}
}

func TestCachedLegStoreWarmsMemoryFromDatabase(t *testing.T) {
	memory := &stubLegStore{}
	stored := &tripmodel.Leg{Polyline: "from-postgres"}
	rows := &stubLegStore{leg: stored}

	leg := find(t, NewCachedLegStore(memory, rows))

	if leg != stored {
		t.Fatalf("leg = %+v", leg)
	}
	if len(memory.upserted) != 1 || memory.upserted[0].Polyline != "from-postgres" {
		t.Fatalf("database hit was not warmed into memory: %+v", memory.upserted)
	}
}

func TestCachedLegStoreMissEverywhere(t *testing.T) {
	memory := &stubLegStore{}
	rows := &stubLegStore{}

	leg := find(t, NewCachedLegStore(memory, rows))

	if leg != nil {
		t.Fatalf("leg = %+v, want miss", leg)
	}
	if len(memory.upserted) != 0 {
		t.Fatal("nothing to warm on a full miss")
	}
}

func TestCachedLegStoreSurvivesBrokenMemory(t *testing.T) {
	memory := &stubLegStore{findErr: errors.New("redis down"), upsertErr: errors.New("redis down")}
	stored := &tripmodel.Leg{Polyline: "from-postgres"}
	rows := &stubLegStore{leg: stored}
	store := NewCachedLegStore(memory, rows)

	leg := find(t, store)
	if leg != stored {
		t.Fatalf("a broken cache must still serve the database row, got %+v", leg)
	}
	if err := store.Upsert(context.Background(), stored); err != nil {
		t.Fatalf("a broken cache must not fail the write: %v", err)
	}
}

func TestCachedLegStoreWritesDatabaseFirst(t *testing.T) {
	// Postgres assigns the row id, so the payload cached afterwards must carry it.
	rowID := uuid.New()
	memory := &stubLegStore{}
	rows := &stubLegStore{assignID: rowID}
	store := NewCachedLegStore(memory, rows)

	if err := store.Upsert(context.Background(), &tripmodel.Leg{Polyline: "computed"}); err != nil {
		t.Fatal(err)
	}
	if len(rows.upserted) != 1 || len(memory.upserted) != 1 {
		t.Fatalf("both layers should be written: rows=%d memory=%d", len(rows.upserted), len(memory.upserted))
	}
	if memory.upserted[0].ID != rowID {
		t.Fatalf("cached leg id = %s, want the id Postgres assigned", memory.upserted[0].ID)
	}
}

func TestCachedLegStoreReturnsDatabaseWriteError(t *testing.T) {
	writeErr := errors.New("postgres down")
	memory := &stubLegStore{}
	rows := &stubLegStore{upsertErr: writeErr}
	store := NewCachedLegStore(memory, rows)

	err := store.Upsert(context.Background(), &tripmodel.Leg{})
	if !errors.Is(err, writeErr) {
		t.Fatalf("err = %v, want %v", err, writeErr)
	}
	if len(memory.upserted) != 0 {
		t.Fatal("a failed database write must not be cached")
	}
}

func TestCachedLegStoreWithoutMemory(t *testing.T) {
	stored := &tripmodel.Leg{Polyline: "from-postgres"}
	rows := &stubLegStore{leg: stored}
	store := NewCachedLegStore(nil, rows)

	if leg := find(t, store); leg != stored {
		t.Fatalf("leg = %+v", leg)
	}
	if err := store.Upsert(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	if len(rows.upserted) != 1 {
		t.Fatal("the database is the only layer left")
	}
}
