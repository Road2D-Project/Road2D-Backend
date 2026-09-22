package repository

import (
	"context"
	"errors"
	"time"

	authenModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type TripMemberRepository struct {
	db *gorm.DB
}

func NewTripMemberRepository(db *gorm.DB) *TripMemberRepository {
	return &TripMemberRepository{db: db}
}

func (r *TripMemberRepository) FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var member model.TripMember
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND trip_id = ?", userId, tripId).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotTripMember
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *TripMemberRepository) FindTripActiveMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) (enum.TripRole, error) {
	if r == nil || r.db == nil {
		return 0, ErrInternalServerError
	}
	var member model.TripMember
	err := r.db.WithContext(ctx).
		Where("trip_id = ? AND user_id = ? AND status = ?", tripId, userId, enum.MembershipActive).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrUserNotTripMember
	}
	if err != nil {
		return 0, err
	}
	return member.Role, nil
}

func (r *TripMemberRepository) ListActiveMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.TripMember
	err := r.db.WithContext(ctx).
		Where("trip_id = ? AND status = ?", tripId, enum.MembershipActive).
		Order("joined_at ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *TripMemberRepository) UpdateMember(ctx context.Context, member *model.TripMember) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	result := r.db.WithContext(ctx).Model(member).
		Select("Role", "Status", "JoinedAt", "Nickname").
		Updates(member)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotTripMember
	}
	return nil
}

func (r *TripMemberRepository) AddActiveMember(ctx context.Context, userId uuid.UUID, tripId uuid.UUID, role enum.TripRole, nickname string) (*model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	ctxDb := r.db.WithContext(ctx)
	var user authenModel.User
	err := ctxDb.Where("id = ?", userId).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotTripMember
	}
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	member := model.TripMember{
		TripID:   tripId,
		UserID:   userId,
		Role:     role,
		Status:   enum.MembershipActive,
		Nickname: nickname,
		JoinedAt: &now,
	}
	if member.Nickname == "" {
		member.Nickname = user.Username
	}
	if err := ctxDb.Create(&member).Error; err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *TripMemberRepository) ApplyLeave(ctx context.Context, leaver *model.TripMember, successor *model.TripMember) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	if leaver == nil {
		return ErrUserNotTripMember
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if successor != nil {
			result := tx.Model(&model.TripMember{}).Where("id = ?", successor.ID).Update("role", enum.TripRoleLeader)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrUserNotTripMember
			}
			result = tx.Model(&model.Trip{}).Where("id = ?", leaver.TripID).Update("owner_id", successor.UserID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrTripNotFound
			}
		}
		result := tx.Model(&model.TripMember{}).Where("id = ?", leaver.ID).Updates(map[string]any{
			"role":      enum.TripRoleMember,
			"status":    enum.MembershipLeft,
			"joined_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUserNotTripMember
		}
		return nil
	})
}
