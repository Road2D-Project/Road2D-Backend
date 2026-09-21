package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

type KickMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	ApplyKick(ctx context.Context, member *model.GroupMember) error
}

type KickRoleCache interface {
	DeleteCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) error
}

type KickMemberService struct {
	members   KickMemberRepository
	roleCache KickRoleCache
}

func NewKickMemberService(members KickMemberRepository, roleCache KickRoleCache) *KickMemberService {
	return &KickMemberService{members: members, roleCache: roleCache}
}

func (s *KickMemberService) Kick(ctx context.Context, actorID, targetID, groupID uuid.UUID, actorRole enum.GroupRole) error {
	if actorID == targetID {
		return repository.ErrCannotKickSelf
	}
	target, err := s.members.FindMemberById(ctx, targetID, groupID)
	if err != nil {
		return err
	}
	if !target.Status.IsActive() {
		return repository.ErrUserNotGroupMember
	}
	if target.Role.IsOwner() {
		return repository.ErrCannotKickOwner
	}
	if actorRole.IsAdmin() && target.Role.IsAdmin() {
		return repository.ErrInsufficientKickRole
	}
	if err := s.members.ApplyKick(ctx, target); err != nil {
		return err
	}
	if s.roleCache != nil {
		_ = s.roleCache.DeleteCachedGroupMemberRole(ctx, groupID, targetID)
	}
	return nil
}
