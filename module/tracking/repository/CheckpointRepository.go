package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CheckpointRepository struct {
	db *gorm.DB
}

func NewCheckpointRepository(db *gorm.DB) *CheckpointRepository {
	return &CheckpointRepository{db: db}
}

func (r *CheckpointRepository) Load(ctx context.Context, tripID uuid.UUID) (*model.TripCheckpoint, error) {
	var row model.CheckpointRecord
	err := r.db.WithContext(ctx).Where("trip_id = ?", tripID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var checkpoint model.TripCheckpoint
	if err := json.Unmarshal([]byte(row.Payload), &checkpoint); err != nil {
		return nil, err
	}
	checkpoint.Sequence = row.Sequence
	return &checkpoint, nil
}

func (r *CheckpointRepository) Save(ctx context.Context, checkpoint model.TripCheckpoint) error {
	payload, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	row := model.CheckpointRecord{
		TripID: checkpoint.TripID, Sequence: checkpoint.Sequence,
		Payload: string(payload), SavedAt: time.Now().UTC(),
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "trip_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"sequence", "payload", "saved_at"}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "tracking_checkpoints.sequence < EXCLUDED.sequence"},
		}},
	}).Create(&row).Error
}
