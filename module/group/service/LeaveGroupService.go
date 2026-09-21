package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"time"

	"github.com/google/uuid"
)

type LeaveMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	ListActiveMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error)
	ApplyLeave(ctx context.Context, leaver *model.GroupMember, successor *model.GroupMember) error
}

type LeaveRoleCache interface {
	SetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID, role enum.GroupRole) error
	DeleteCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) error
}

type LeaveGroupService struct {
	members   LeaveMemberRepository
	roleCache LeaveRoleCache
}

func NewLeaveGroupService(members LeaveMemberRepository, roleCache LeaveRoleCache) *LeaveGroupService {
	return &LeaveGroupService{members: members, roleCache: roleCache}
}

func (s *LeaveGroupService) Leave(ctx context.Context, userId, groupId uuid.UUID) error {
	leaver, err := s.members.FindMemberById(ctx, userId, groupId)
	if err != nil {
		return err
	}
	if !leaver.Status.IsActive() {
		return repository.ErrUserNotGroupMember
	}

	var successor *model.GroupMember
	if leaver.Role.IsOwner() {
		roster, err := s.members.ListActiveMembers(ctx, groupId)
		if err != nil {
			return err
		}
		successor = pickOwnershipSuccessor(roster, userId)
		if successor == nil {
			return repository.ErrNoSuccessorToTransfer
		}
	}

	if err := s.members.ApplyLeave(ctx, leaver, successor); err != nil {
		return err
	}
	if s.roleCache == nil {
		return nil
	}
	_ = s.roleCache.DeleteCachedGroupMemberRole(ctx, groupId, userId)
	// trao quyền và cập nhật cache
	if successor != nil {
		_ = s.roleCache.SetCachedGroupMemberRole(ctx, groupId, successor.UserID, enum.GroupRoleOwner)
	}
	return nil
}

func pickOwnershipSuccessor(members []model.GroupMember, leavingUserID uuid.UUID) *model.GroupMember {
	var earliestAdmin, earliestMember *model.GroupMember
	for i := range members {
		m := &members[i]
		if m.UserID == leavingUserID || !m.Status.IsActive() || m.Role.IsOwner() {
			continue
		}
		if m.Role.IsAdmin() {
			if joinedEarlier(m, earliestAdmin) {
				earliestAdmin = m
			}
			continue
		}
		if joinedEarlier(m, earliestMember) {
			earliestMember = m
		}
	}
	if earliestAdmin != nil {
		return earliestAdmin
	}
	return earliestMember
}

// so sánh ai vào sớm hơn
func joinedEarlier(candidate, current *model.GroupMember) bool {
	if current == nil {
		return true
	}
	return joinedAtOrZero(candidate).Before(joinedAtOrZero(current))
}

// chuẩn hóa, lỗi thì đặt thời gian đến vô tận
func joinedAtOrZero(member *model.GroupMember) time.Time {
	if member == nil || member.JoinedAt == nil {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return *member.JoinedAt
}
