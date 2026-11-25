package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TripImageRepository stores the trip image key and satisfies objectStorage.ImageRepo.
type TripImageRepository struct {
	db *gorm.DB
}

var _ objectStorage.ImageRepo = (*TripImageRepository)(nil)

func NewTripImageRepository(db *gorm.DB) *TripImageRepository {
	return &TripImageRepository{db: db}
}

func (r *TripImageRepository) SetImagePath(ctx context.Context, objectID, newPath string) (string, error) {
	trip, err := r.trip(ctx, objectID)
	if err != nil {
		return "", err
	}
	if err := objectStorage.ValidateOwnedPath(objectID, newPath); err != nil {
		return "", err
	}
	oldKey := ""
	if trip.ImagePath != nil {
		oldKey = *trip.ImagePath
	}
	err = r.db.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", trip.ID).Update("image_url", newPath).Error
	if err != nil {
		return "", err
	}
	return oldKey, nil
}

func (r *TripImageRepository) ImagePath(ctx context.Context, objectID string) (string, error) {
	trip, err := r.trip(ctx, objectID)
	if err != nil {
		return "", err
	}
	if trip.ImagePath == nil || *trip.ImagePath == "" {
		return "", nil
	}
	if err := objectStorage.ValidateOwnedPath(objectID, *trip.ImagePath); err != nil {
		return "", err
	}
	return *trip.ImagePath, nil
}

func (r *TripImageRepository) ClearImagePath(ctx context.Context, objectID string) error {
	trip, err := r.trip(ctx, objectID)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", trip.ID).Update("image_url", nil).Error
}

func (r *TripImageRepository) trip(ctx context.Context, objectID string) (*model.Trip, error) {
	id, err := uuid.Parse(objectID)
	if err != nil {
		return nil, err
	}
	var trip model.Trip
	err = r.db.WithContext(ctx).Select("id", "image_url").Where("id = ?", id).First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}
