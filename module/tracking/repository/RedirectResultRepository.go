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

type RedirectResultRepository struct {
	db *gorm.DB
}

func NewRedirectResultRepository(db *gorm.DB) *RedirectResultRepository {
	return &RedirectResultRepository{db: db}
}

func (r *RedirectResultRepository) Find(ctx context.Context, jobID uuid.UUID) (*model.RedirectResult, error) {
	var row model.RedirectResultRecord
	err := r.db.WithContext(ctx).Where("job_id = ?", jobID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result model.RedirectResult
	if err := json.Unmarshal([]byte(row.Payload), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (r *RedirectResultRepository) Save(ctx context.Context, result model.RedirectResult) (*model.RedirectResult, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	row := model.RedirectResultRecord{
		JobID: result.JobID, Payload: string(payload), CompletedAt: time.Now().UTC(),
	}
	err = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	if err != nil {
		return nil, err
	}
	return r.Find(ctx, result.JobID)
}
