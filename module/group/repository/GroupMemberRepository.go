package repository

import (
	"context"
	"errors"
	"time"

	authenModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type GroupMemberRepository struct {
	db *gorm.DB
}

func NewGroupMemberRepository(db *gorm.DB) *GroupMemberRepository {
	return &GroupMemberRepository{db: db}
}

func (r *GroupMemberRepository) ListInvitedMembers(ctx context.Context, userId uuid.UUID) ([]model.GroupMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.GroupMember
	err := r.db.WithContext(ctx).
		Preload("Group").
		Preload("User").
		Where("user_id = ? AND status = ?", userId, enum.MembershipInvited).
		Order("updated_at DESC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *GroupMemberRepository) FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var member model.GroupMember
	err := r.db.WithContext(ctx).
		Preload("Group").
		Preload("User").
		Where("user_id = ? AND group_id = ?", userId, groupId).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotGroupMember
	}
	if err != nil {
		return nil, err
	}
	return &member, nil
}

func (r *GroupMemberRepository) UpdateMember(ctx context.Context, member *model.GroupMember) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	result := r.db.WithContext(ctx).Model(member).
		Select("Role", "Status", "InvitorName", "JoinedAt", "Nickname").
		Updates(member)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrUserNotGroupMember
	}
	return nil
}

func (r *GroupMemberRepository) InviteMember(ctx context.Context, userId uuid.UUID, groupId uuid.UUID, invitorName string) (*model.GroupMember, error) {
	return r.AddNewMember(ctx, userId, groupId, enum.GroupRoleMember, enum.MembershipInvited, &invitorName)
}

func (r *GroupMemberRepository) RequestJoin(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error) {
	return r.AddNewMember(ctx, userId, groupId, enum.GroupRoleMember, enum.MembershipPending, nil)
}

func (r *GroupMemberRepository) ListPendingMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.GroupMember
	err := r.db.WithContext(ctx).
		Preload("Group").
		Preload("User").
		Where("group_id = ? AND status = ?", groupId, enum.MembershipPending).
		Order("updated_at DESC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *GroupMemberRepository) ListActiveMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error) {
	if r == nil || r.db == nil {
		return nil, ErrInternalServerError
	}
	var members []model.GroupMember
	err := r.db.WithContext(ctx).
		Preload("User").
		Where("group_id = ? AND status = ?", groupId, enum.MembershipActive).
		Order("joined_at ASC").
		Find(&members).Error
	if err != nil {
		return nil, err
	}
	return members, nil
}

func (r *GroupMemberRepository) FindGroupActiveMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error) {
	if r == nil || r.db == nil {
		return 0, ErrInternalServerError
	}
	var member model.GroupMember
	err := r.db.WithContext(ctx).
		Where("group_id = ? AND user_id = ? AND status = ?", groupId, userId, enum.MembershipActive).
		First(&member).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, ErrUserNotGroupMember
	}
	if err != nil {
		return 0, err
	}
	return member.Role, nil
}

func (r *GroupMemberRepository) AddNewMember(ctx context.Context, userId uuid.UUID, groupId uuid.UUID, role enum.GroupRole, status enum.MembershipStatus, invitorName *string) (*model.GroupMember, error) {
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
	var group model.Group
	err = ctxDb.Where("id = ?", groupId).First(&group).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrGroupNotFound
	}
	if err != nil {
		return nil, err
	}
	member := model.GroupMember{
		GroupID:     groupId,
		UserID:      userId,
		Role:        role,
		Status:      status,
		Nickname:    user.Username,
		InvitorName: invitorName,
	}
	if status.IsActive() {
		joinTime := time.Now().UTC()
		member.JoinedAt = &joinTime
	}
	if err := ctxDb.Create(&member).Error; err != nil {
		return nil, err
	}
	member.Group = &group
	member.User = &user
	return &member, nil
}

func (r *GroupMemberRepository) ApplyLeave(ctx context.Context, leaver *model.GroupMember, successor *model.GroupMember) error {
	if r == nil || r.db == nil {
		return ErrInternalServerError
	}
	if leaver == nil {
		return ErrUserNotGroupMember
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if successor != nil {
			// update quyền
			result := tx.Model(&model.GroupMember{}).Where("id = ?", successor.ID).Update("role", enum.GroupRoleOwner)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrUserNotGroupMember
			}
			// update group
			result = tx.Model(&model.Group{}).Where("id = ?", leaver.GroupID).Update("owner_id", successor.UserID)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return ErrGroupNotFound
			}
		}
		result := tx.Model(&model.GroupMember{}).Where("id = ?", leaver.ID).Updates(map[string]any{
			"role":      enum.GroupRoleMember,
			"status":    enum.MembershipLeft,
			"joined_at": nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrUserNotGroupMember
		}
		return nil
	})
}
