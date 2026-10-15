package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/objectStorage"
	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const tripImageBucket = "trip"

// TripImagePaths is the object key for a trip picture.
// The key is <tripID>/<tripID><ext> inside the trip bucket.
type TripImagePaths struct{}

func (TripImagePaths) Bucket() string { return tripImageBucket }

func (TripImagePaths) ObjectPath(objectID, ext string) string {
	return objectID + "/" + objectID + ext
}

// TripImageRepository stores the public trip image URL and satisfies objectStorage.ImageRepo.
type TripImageRepository struct {
	db     *gorm.DB
	images *objectStorage.Client
}

var _ objectStorage.ImageRepo = (*TripImageRepository)(nil)

func NewTripImageRepository(db *gorm.DB) *TripImageRepository {
	return &TripImageRepository{db: db, images: objectStorage.NewClient()}
}

func (r *TripImageRepository) SetImagePath(ctx context.Context, objectID, newPath string) (string, error) {
	trip, err := r.trip(ctx, objectID)
	if err != nil {
		return "", err
	}
	newURL, err := r.images.PublicObjectURL(tripImageBucket, newPath)
	if err != nil {
		return "", err
	}
	oldKey := ""
	if trip.ImageURL != nil {
		if key, ok := objectStorage.PublicObjectPath(*trip.ImageURL, tripImageBucket); ok {
			oldKey = key
		}
	}
	err = r.db.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", trip.ID).Update("image_url", newURL).Error
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
	if trip.ImageURL == nil || *trip.ImageURL == "" {
		return "", nil
	}
	key, ok := objectStorage.PublicObjectPath(*trip.ImageURL, tripImageBucket)
	if !ok {
		return "", objectStorage.ErrInvalidPath
	}
	return key, nil
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
