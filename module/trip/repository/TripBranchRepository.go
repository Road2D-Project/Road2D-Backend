package repository

import (
	"Road-To-Destination-BE/module/trip/model"
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TripBranchRepository struct {
	db *gorm.DB
}

// FindDestinationsByIDs loads every destination the graph references in one query.
func (r *TripBranchRepository) FindDestinationsByIDs(ctx context.Context, ids []uuid.UUID) ([]model.Destination, error) {
	if len(ids) == 0 {
		return []model.Destination{}, nil
	}
	var destinations []model.Destination
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&destinations).Error; err != nil {
		return nil, err
	}
	return destinations, nil
}

// ReplaceTripBranches drops the trip's current branches and writes the new
// ones. Branch stops cascade with the deleted rows, so the graph is replaced
// as a whole inside one transaction.
func (r *TripBranchRepository) ReplaceTripBranches(ctx context.Context, tripID uuid.UUID, branches []model.TripBranch) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("trip_id = ?", tripID).Delete(&model.TripBranch{}).Error; err != nil {
			return err
		}
		if len(branches) == 0 {
			return nil
		}
		for i := range branches {
			branches[i].TripID = tripID
		}
		return tx.Create(&branches).Error
	})
}

// FindTripWithBranches loads the trip with its branches, ordered stops and the
// destination behind each stop.
func (r *TripBranchRepository) FindTripWithBranches(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var trip model.Trip
	err := r.db.WithContext(ctx).
		Preload("Branches.Stops", func(db *gorm.DB) *gorm.DB {
			return db.Order("order_in_branch ASC")
		}).
		Preload("Branches.Stops.Destination").
		Where("id = ?", id).
		First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func NewTripBranchRepository(db *gorm.DB) *TripBranchRepository {
	return &TripBranchRepository{db: db}
}
