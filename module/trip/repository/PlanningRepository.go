package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DestinationUsage includes historical travel references even after a graph edit
// has removed the pin from every branch.
type DestinationUsage struct {
	TripID    uuid.UUID
	OnRoute   bool
	HasTravel bool
	Frozen    bool
}

// PlanningRepository binds the existing repositories to one transaction.
type PlanningRepository struct {
	*DestinationRepository
	*LocationRepository
	*TripBranchRepository
	*TripRepository
	*TripMemberRepository
	*TravelRepository
	database *gorm.DB
}

func NewPlanningRepository(db *gorm.DB) *PlanningRepository {
	return &PlanningRepository{
		DestinationRepository: NewDestinationRepository(db),
		LocationRepository:    NewLocationRepository(db),
		TripBranchRepository:  NewTripBranchRepository(db),
		TripRepository:        NewTripRepository(db),
		TripMemberRepository:  NewTripMemberRepository(db),
		TravelRepository:      NewTravelRepository(db),
		database:              db,
	}
}

// Serializable isolation protects reference-set changes as well as row updates.
// Callbacks contain database work only, so aborted attempts can be retried safely.
func (r *PlanningRepository) Transaction(ctx context.Context, run func(*PlanningRepository) error) error {
	for attempt := 0; attempt < 3; attempt++ {
		err := r.database.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return run(NewPlanningRepository(tx))
		}, &sql.TxOptions{Isolation: sql.LevelSerializable})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "40001" && pgErr.Code != "40P01") {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return ErrPlanningConflict
}

func (r *PlanningRepository) FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var trip model.Trip
	err := r.database.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&trip, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	return &trip, err
}

func (r *PlanningRepository) FindTripWithBranches(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	if _, err := r.FindTripByID(ctx, id); err != nil {
		return nil, err
	}
	return r.TripBranchRepository.FindTripWithBranches(ctx, id)
}

func (r *PlanningRepository) FindDestinationById(ctx context.Context, id uuid.UUID) (*model.Destination, error) {
	var pin model.Destination
	err := r.database.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Preload("Location").Preload("Location.PlaceType").First(&pin, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrDestinationNotFound
	}
	return &pin, err
}

func (r *PlanningRepository) FindDestinationsByIDs(ctx context.Context, ids []uuid.UUID) ([]model.Destination, error) {
	var pins []model.Destination
	if len(ids) == 0 {
		return pins, nil
	}
	err := r.database.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).Order("id").Find(&pins).Error
	return pins, err
}

func (r *PlanningRepository) FindTripActiveMemberRole(ctx context.Context, tripID, userID uuid.UUID) (enum.TripRole, error) {
	var member model.TripMember
	err := r.database.WithContext(ctx).Clauses(clause.Locking{Strength: "SHARE"}).
		Where("trip_id = ? AND user_id = ? AND status = ?", tripID, userID, enum.MembershipActive).First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrUserNotTripMember
	}
	return member.Role, err
}

func (r *PlanningRepository) DestinationUsages(ctx context.Context, id uuid.UUID) ([]DestinationUsage, error) {
	var usages []DestinationUsage
	err := r.database.WithContext(ctx).Raw(`
		SELECT t.id AS trip_id,
		 EXISTS (SELECT 1 FROM branch_destinations bd JOIN trip_branches b ON b.id = bd.trip_branch_id
		         WHERE b.trip_id = t.id AND bd.destination_id = ? AND NOT b.is_draft) AS on_route,
		 EXISTS (SELECT 1 FROM travels tr WHERE tr.trip_id = t.id
		         AND (tr.from_destination_id = ? OR tr.to_destination_id = ?)) AS has_travel,
		 EXISTS (SELECT 1 FROM travels tr WHERE tr.trip_id = t.id AND tr.is_frozen
		         AND (tr.from_destination_id = ? OR tr.to_destination_id = ?)) AS frozen
		FROM trips t
		JOIN (
		 SELECT b.trip_id FROM branch_destinations bd JOIN trip_branches b ON b.id = bd.trip_branch_id
		 WHERE bd.destination_id = ?
		 UNION
		 SELECT tr.trip_id FROM travels tr WHERE tr.from_destination_id = ? OR tr.to_destination_id = ?
		) affected ON affected.trip_id = t.id
		ORDER BY t.id`, id, id, id, id, id, id, id, id).Scan(&usages).Error
	return usages, err
}

func (r *PlanningRepository) InvalidateDestinationTravels(ctx context.Context, id uuid.UUID) error {
	return r.database.WithContext(ctx).Where("NOT is_frozen AND (from_destination_id = ? OR to_destination_id = ?)", id, id).
		Delete(&model.Travel{}).Error
}

func (r *PlanningRepository) TouchTrip(ctx context.Context, id uuid.UUID) error {
	return r.database.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", id).
		Update("updated_at", time.Now().UTC()).Error
}
