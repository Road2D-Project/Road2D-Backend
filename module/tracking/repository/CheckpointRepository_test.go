package repository

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func checkpointTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: "host=127.0.0.1 port=5432 user=test dbname=test sslmode=disable",
	}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestCheckpointUpsertGuardsAgainstLateOlderWrite(t *testing.T) {
	db := checkpointTestDatabase(t)
	var statement string
	var saved model.CheckpointRecord
	if err := db.Callback().Create().After("gorm:create").Register("capture-checkpoint", func(tx *gorm.DB) {
		statement = tx.Statement.SQL.String()
		saved = *tx.Statement.Dest.(*model.CheckpointRecord)
	}); err != nil {
		t.Fatal(err)
	}
	checkpoint := model.TripCheckpoint{TripID: uuid.New(), Sequence: 8, Revision: 12}
	if err := NewCheckpointRepository(db).Save(context.Background(), checkpoint); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(statement, `ON CONFLICT ("trip_id") DO UPDATE`) ||
		!strings.Contains(statement, "tracking_checkpoints.sequence < EXCLUDED.sequence") {
		t.Fatalf("upsert permits stale replacement: %s", statement)
	}
	var payload model.TripCheckpoint
	if err := json.Unmarshal([]byte(saved.Payload), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.TripID != checkpoint.TripID || payload.Sequence != saved.Sequence || payload.Revision != 12 {
		t.Fatalf("durable envelope and snapshot disagree: %+v", saved)
	}
}

func TestCheckpointSavePropagatesDatabaseError(t *testing.T) {
	db := checkpointTestDatabase(t)
	writeError := errors.New("checkpoint insert failed")
	if err := db.Callback().Create().Replace("gorm:create", func(tx *gorm.DB) {
		tx.AddError(writeError)
	}); err != nil {
		t.Fatal(err)
	}
	err := NewCheckpointRepository(db).Save(context.Background(), model.TripCheckpoint{TripID: uuid.New()})
	if !errors.Is(err, writeError) {
		t.Fatalf("got %v, want write error", err)
	}
}
