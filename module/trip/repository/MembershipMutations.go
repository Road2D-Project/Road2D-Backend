package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/share/outbox"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TripAccessLifecycle is implemented by the live transport at composition time.
// Domain persistence never imports chatting or operates sockets directly.
type TripAccessLifecycle interface {
	TripChange(context.Context, uuid.UUID, func(context.Context) error) error
}

func (r *TripMemberRepository) WithAccessLifecycle(lifecycle TripAccessLifecycle) *TripMemberRepository {
	r.lifecycle = lifecycle
	return r
}

func (r *TripMemberRepository) mutate(ctx context.Context, tripID uuid.UUID, removed []uuid.UUID, run func(*TripMemberRepository) error) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	change := func(ctx context.Context) error {
		return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var trip model.Trip
			if err := tx.Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).First(&trip, "id = ?", tripID).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrTripNotFound
				}
				return err
			}
			var formerlyActive []uuid.UUID
			if len(removed) > 0 {
				if err := tx.Model(&model.TripMember{}).Where("trip_id = ? AND user_id IN ? AND status = 'active'", tripID, removed).Pluck("user_id", &formerlyActive).Error; err != nil {
					return err
				}
			}
			if err := run(&TripMemberRepository{db: tx}); err != nil {
				return err
			}
			return outbox.AppendRosterChange(tx, tripID, formerlyActive)
		})
	}
	if r.lifecycle != nil {
		return r.lifecycle.TripChange(ctx, tripID, change)
	}
	return change(ctx)
}

func (r *TripMemberRepository) UpdateMember(ctx context.Context, member *model.TripMember) error {
	if member == nil {
		return ErrUserNotTripMember
	}
	var removed []uuid.UUID
	if !member.Status.IsActive() {
		removed = []uuid.UUID{member.UserID}
	}
	return r.mutate(ctx, member.TripID, removed, func(tx *TripMemberRepository) error { return tx.updateMember(ctx, member) })
}

func (r *TripMemberRepository) AddMember(ctx context.Context, userID, tripID uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, invitor *string) (*model.TripMember, error) {
	var member *model.TripMember
	err := r.mutate(ctx, tripID, nil, func(tx *TripMemberRepository) error {
		var err error
		member, err = tx.addMember(ctx, userID, tripID, role, status, nickname, invitor)
		return err
	})
	return member, err
}

func (r *TripMemberRepository) ApplyKick(ctx context.Context, member *model.TripMember) error {
	if member == nil {
		return ErrUserNotTripMember
	}
	return r.mutate(ctx, member.TripID, []uuid.UUID{member.UserID}, func(tx *TripMemberRepository) error { return tx.applyKick(ctx, member) })
}

func (r *TripMemberRepository) ApplyLeave(ctx context.Context, member, successor *model.TripMember) error {
	if member == nil {
		return ErrUserNotTripMember
	}
	return r.mutate(ctx, member.TripID, []uuid.UUID{member.UserID}, func(tx *TripMemberRepository) error { return tx.applyLeave(ctx, member, successor) })
}
