package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ActiveTripByUser is a trip plus the caller's role on that trip.
type ActiveTripByUser struct {
	Trip model.Trip
	Role enum.TripRole
}

type TripRepository struct {
	db *gorm.DB
}

func NewTripRepository(db *gorm.DB) *TripRepository {
	return &TripRepository{db: db}
}

func (r *TripRepository) CreateTripWithMembers(ctx context.Context, trip *model.Trip, members []model.TripMember) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(trip).Error; err != nil {
			return err
		}
		for i := range members {
			members[i].TripID = trip.ID
		}
		if len(members) == 0 {
			return ErrUserNotTripMember
		}
		return tx.Create(&members).Error
	})
}

func (r *TripRepository) FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	var trip model.Trip
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepository) FindTripByInviteToken(ctx context.Context, token string) (*model.Trip, error) {
	var trip model.Trip
	err := r.db.WithContext(ctx).Where("invite_token = ?", token).First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInviteNotFound
	}
	if err != nil {
		return nil, err
	}
	return &trip, nil
}

func (r *TripRepository) UpdateTripInfo(ctx context.Context, id uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return ErrNoTripUpdate
	}
	result := r.db.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTripNotFound
	}
	return nil
}

func (r *TripRepository) UpdateInviteToken(ctx context.Context, id uuid.UUID, token string) error {
	result := r.db.WithContext(ctx).Model(&model.Trip{}).Where("id = ?", id).Update("invite_token", token)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTripNotFound
	}
	return nil
}

func (r *TripRepository) DeleteTrip(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&model.Trip{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrTripNotFound
	}
	return nil
}

func (r *TripRepository) ListActiveTripByUser(ctx context.Context, userID uuid.UUID) ([]ActiveTripByUser, error) {
	var members []model.TripMember
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, enum.MembershipActive).
		Find(&members).Error; err != nil {
		return nil, err
	}
	if len(members) == 0 {
		return []ActiveTripByUser{}, nil
	}
	ids := make([]uuid.UUID, 0, len(members))
	roleByTrip := make(map[uuid.UUID]enum.TripRole, len(members))
	for _, m := range members {
		ids = append(ids, m.TripID)
		roleByTrip[m.TripID] = m.Role
	}
	var trips []model.Trip
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Order("updated_at DESC").Find(&trips).Error; err != nil {
		return nil, err
	}
	out := make([]ActiveTripByUser, 0, len(trips))
	for _, trip := range trips {
		out = append(out, ActiveTripByUser{Trip: trip, Role: roleByTrip[trip.ID]})
	}
	return out, nil
}
