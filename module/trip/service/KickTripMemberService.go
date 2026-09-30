package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"

	"github.com/google/uuid"
)

type KickTripStore interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	ApplyKick(ctx context.Context, member *model.TripMember) error
}

type KickTripRoleCache interface {
	DeleteCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) error
}

type KickTripMemberService struct {
	members   KickTripStore
	roleCache KickTripRoleCache
}

func NewKickTripMemberService(members KickTripStore, roleCache KickTripRoleCache) *KickTripMemberService {
	return &KickTripMemberService{members: members, roleCache: roleCache}
}

func (s *KickTripMemberService) Kick(ctx context.Context, actorID, targetID, tripID uuid.UUID, actorRole enum.TripRole) error {
	if !actorRole.CanManageMembers() {
		return repository.ErrInsufficientKickRole
	}
	if actorID == targetID {
		return repository.ErrCannotKickSelf
	}
	target, err := s.members.FindMemberById(ctx, targetID, tripID)
	if err != nil {
		return err
	}
	if !target.Status.IsActive() {
		return repository.ErrUserNotTripMember
	}
	if target.Role.IsLeader() {
		return repository.ErrCannotKickLeader
	}
	if actorRole.IsAdmin() && target.Role.IsAdmin() {
		return repository.ErrInsufficientKickRole
	}
	if err := s.members.ApplyKick(ctx, target); err != nil {
		return err
	}
	if s.roleCache != nil {
		_ = s.roleCache.DeleteCachedTripMemberRole(ctx, tripID, targetID)
	}
	return nil
}
