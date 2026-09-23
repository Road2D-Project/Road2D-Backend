package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DestinationRepository struct {
	db *gorm.DB
}

func NewDestinationRepository(db *gorm.DB) *DestinationRepository {
	return &DestinationRepository{db: db}
}

func (r *DestinationRepository) FindDestinationById(ctx context.Context, destinationId uuid.UUID) (*model.Destination, error) {
	var destination model.Destination
	err := r.db.WithContext(ctx).
		Preload("Location").
		Preload("Location.PlaceType").
		Where("id = ?", destinationId).
		First(&destination).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDestinationNotFound
	}
	if err != nil {
		return nil, err
	}
	return &destination, nil
}

// CreateDestination inserts the pin only. The linked Location, when set, already exists.
func (r *DestinationRepository) CreateDestination(ctx context.Context, destination *model.Destination) error {
	return r.db.WithContext(ctx).Omit("Location").Create(destination).Error
}

func (r *DestinationRepository) UpdateDestination(ctx context.Context, destination *model.Destination) error {
	result := r.db.WithContext(ctx).
		Model(&model.Destination{}).
		Where("id = ?", destination.ID).
		Updates(map[string]any{
			"lat":            destination.Lat,
			"lng":            destination.Lng,
			"name":           destination.Name,
			"arrive_time":    destination.ArriveTime,
			"stay_over_time": destination.StayOverTime,
			"status":         destination.Status,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDestinationNotFound
	}
	return nil
}
