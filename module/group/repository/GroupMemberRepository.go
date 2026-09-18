package repository

import (
	"context"
	"errors"

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

func (r *GroupMemberRepository) FindGroupActiveMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error) {
	if r == nil || r.db == nil {
		return 0, ErrUserNotGroupMember
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
