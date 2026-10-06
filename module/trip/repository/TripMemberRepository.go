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

func (r *TripMemberRepository) AssignBranch(ctx context.Context, member *model.TripMember, branchID uuid.UUID) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	if member == nil || member.ID == uuid.Nil {
		return ErrUserNotTripMember
	}
	var branch model.TripBranch
	err := r.db.WithContext(ctx).
		Where("id = ? AND trip_id = ?", branchID, member.TripID).
		First(&branch).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrBranchNotFound
	}
	if err != nil {
		return err
	}
	if !branch.OnRoute() {
		return ErrDraftBranchExcluded
	}
	result := r.db.WithContext(ctx).Model(&model.TripMember{}).
		Where("id = ?", member.ID).
		Update("assigned_branch_id", branch.ID)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotTripMember
	}
	member.AssignedBranchID = &branch.ID
	member.AssignedBranch = &branch
	return nil
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
		Preload("Trip").
		Preload("User").
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
		Preload("User").
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
		Select("Role", "Status", "JoinedAt", "Nickname", "InvitorName").
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
	return r.AddMember(ctx, userId, tripId, role, enum.MembershipActive, nickname, nil)
}

func (r *TripMemberRepository) AddMember(ctx context.Context, userId uuid.UUID, tripId uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, invitorName *string) (*model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	ctxDb := r.db.WithContext(ctx)
	var user authenModel.User
	err := ctxDb.Where("id = ?", userId).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	var trip model.Trip
	err = ctxDb.Where("id = ?", tripId).First(&trip).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrTripNotFound
	}
	if err != nil {
		return nil, err
	}
	member := model.TripMember{
		TripID:      tripId,
		UserID:      userId,
		Role:        role,
		Status:      status,
		Nickname:    nickname,
		InvitorName: invitorName,
	}
	if member.Nickname == "" {
		member.Nickname = user.Username
	}
	if status.IsActive() {
		now := time.Now().UTC()
		member.JoinedAt = &now
	}
	if err := ctxDb.Create(&member).Error; err != nil {
		return nil, err
	}
	member.User = &user
	member.Trip = &trip
	return &member, nil
}

func (r *TripMemberRepository) CountActiveMembers(ctx context.Context, tripId uuid.UUID) (int, error) {
	if r == nil || r.db == nil {
		return 0, ErrInternalServerError
	}
	var n int64
	err := r.db.WithContext(ctx).Model(&model.TripMember{}).
		Where("trip_id = ? AND status = ?", tripId, enum.MembershipActive).
		Count(&n).Error
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *TripMemberRepository) ListPendingMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.TripMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Trip").
		Where("trip_id = ? AND status = ?", tripId, enum.MembershipPending).
		Order("updated_at DESC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *TripMemberRepository) ListInvitedMembers(ctx context.Context, userId uuid.UUID) ([]model.TripMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.TripMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Preload("Trip").
		Where("user_id = ? AND status = ?", userId, enum.MembershipInvited).
		Order("updated_at DESC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *TripMemberRepository) ApplyKick(ctx context.Context, member *model.TripMember) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	if member == nil {
		return ErrUserNotTripMember
	}
	result := r.db.WithContext(ctx).Model(&model.TripMember{}).Where("id = ?", member.ID).Updates(map[string]any{
		"role":      enum.TripRoleMember,
		"status":    enum.MembershipKicked,
		"joined_at": nil,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotTripMember
	}
	return nil
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
