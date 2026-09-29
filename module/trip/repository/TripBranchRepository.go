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

// ReplaceTripBranches drops the trip's route branches and writes the new
// ones. The draft inbox stays: replacing the ride must not throw away pins
// that have not been placed yet. Route-branch stops cascade with the deleted rows.
func (r *TripBranchRepository) ReplaceTripBranches(ctx context.Context, tripID uuid.UUID, branches []model.TripBranch) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("trip_id = ? AND is_draft = ?", tripID, false).Delete(&model.TripBranch{}).Error; err != nil {
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

// UpdateBranchStops rewrites one branch's split, merge, and ordered stops.
// The branch row itself stays, so its id does not change. Stops are replaced
// inside the transaction: the unique (branch, destination) and (branch, order)
// indexes would reject an insert that still overlaps the old rows.
func (r *TripBranchRepository) UpdateBranchStops(ctx context.Context, branch model.TripBranch) error {
	stops := make([]model.BranchDestination, len(branch.Stops))
	for i, stop := range branch.Stops {
		stops[i] = model.BranchDestination{
			TripBranchID:  branch.ID,
			DestinationID: stop.DestinationID,
			OrderInBranch: stop.OrderInBranch,
		}
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TripBranch{}).
			Where("id = ? AND trip_id = ?", branch.ID, branch.TripID).
			Updates(map[string]any{
				"split_from_destination_id": nullableUUID(branch.SplitFromDestinationID),
				"merge_to_destination_id":   nullableUUID(branch.MergeToDestinationID),
			}).Error; err != nil {
			return err
		}
		if err := tx.Where("trip_branch_id = ?", branch.ID).Delete(&model.BranchDestination{}).Error; err != nil {
			return err
		}
		if len(stops) == 0 {
			return nil
		}
		return tx.Create(&stops).Error
	})
}

// ListTripIDsByDestination returns each trip that still has this pin on a branch.
// A pin can sit on more than one trip, so delete has to look past the trip in the URL.
func (r *TripBranchRepository) ListTripIDsByDestination(ctx context.Context, destinationID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		TripID uuid.UUID `gorm:"column:trip_id"`
	}
	err := r.db.WithContext(ctx).Raw(
		`SELECT DISTINCT tb.trip_id
		 FROM branch_destinations AS bd
		 JOIN trip_branches AS tb ON tb.id = bd.trip_branch_id
		 WHERE bd.destination_id = ?`,
		destinationID,
	).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].TripID
	}
	return ids, nil
}

// DeleteDestination removes the pin. Travels that still point at it cascade with the row.
// A pin that lives only on the draft inbox can be deleted here: those stops cascade too.
// A pin on a route branch must already have been stripped. Cascading those rows would
// leave gaps in order_in_branch and a split or merge pointing nowhere.
func (r *TripBranchRepository) DeleteDestination(ctx context.Context, destinationID uuid.UUID) error {
	result := r.db.WithContext(ctx).Where("id = ?", destinationID).Delete(&model.Destination{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDestinationNotFound
	}
	return nil
}

func nullableUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

func NewTripBranchRepository(db *gorm.DB) *TripBranchRepository {
	return &TripBranchRepository{db: db}
}
