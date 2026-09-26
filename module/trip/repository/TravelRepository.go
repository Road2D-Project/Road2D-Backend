package repository

import (
	"context"

	"Road-To-Destination-BE/module/trip/model"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TravelRepository struct {
	db *gorm.DB
}

func NewTravelRepository(db *gorm.DB) *TravelRepository {
	return &TravelRepository{db: db}
}

// FindTravelsByTrip loads every computed leg of the trip in one query; a trip
// holds a few dozen at most, so the caller indexes them in memory.
func (r *TravelRepository) FindTravelsByTrip(ctx context.Context, tripID uuid.UUID) ([]model.Travel, error) {
	var travels []model.Travel
	if err := r.db.WithContext(ctx).Where("trip_id = ?", tripID).Find(&travels).Error; err != nil {
		return nil, err
	}
	return travels, nil
}

// UpsertTravels writes the freshly computed legs on the idx_travel_pair key.
// A frozen row is trip history: the conflict clause skips it instead of
// overwriting the snapshot someone may already have shared.
func (r *TravelRepository) UpsertTravels(ctx context.Context, tripID uuid.UUID, travels []model.Travel) error {
	if len(travels) == 0 {
		return nil
	}
	rows := make([]model.Travel, len(travels))
	copy(rows, travels)
	for i := range rows {
		rows[i].TripID = tripID
	}
	return r.db.WithContext(ctx).
		Omit("Trip", "FromDestination", "ToDestination", "Leg").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "trip_id"},
				{Name: "from_destination_id"},
				{Name: "to_destination_id"},
				{Name: "vehicle"},
			},
			Where: clause.Where{Exprs: []clause.Expression{
				clause.Eq{Column: clause.Column{Table: "travels", Name: "is_frozen"}, Value: false},
			}},
			DoUpdates: clause.AssignmentColumns([]string{
				"leg_id", "polyline", "distance_m", "duration_s", "last_computed_at", "updated_at",
			}),
		}).
		Create(&rows).Error
}
