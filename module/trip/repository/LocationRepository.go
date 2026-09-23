package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LocationRepository reads verified places. A location is never updated here:
// planning copies one by forking it into a destination.
type LocationRepository struct {
	db *gorm.DB
}

func NewLocationRepository(db *gorm.DB) *LocationRepository {
	return &LocationRepository{db: db}
}

func (r *LocationRepository) FindLocationById(ctx context.Context, locationId uuid.UUID) (*model.Location, error) {
	var location model.Location
	err := r.db.WithContext(ctx).Preload("PlaceType").Where("id = ?", locationId).First(&location).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLocationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &location, nil
}

func (r *LocationRepository) FindLocationByPlaceID(ctx context.Context, placeID string) (*model.Location, error) {
	if placeID == "" {
		return nil, ErrLocationNotFound
	}
	var location model.Location
	err := r.db.WithContext(ctx).Where("place_id = ?", placeID).First(&location).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrLocationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &location, nil
}

func (r *LocationRepository) CreateLocation(ctx context.Context, location *model.Location) error {
	return r.db.WithContext(ctx).Omit("PlaceType").Create(location).Error
}
