package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"time"

	"github.com/google/uuid"
)

type LeaveTripMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	ListActiveMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error)
	ApplyLeave(ctx context.Context, leaver *model.TripMember, successor *model.TripMember) error
}

type LeaveTripRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
	DeleteCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) error
}

type LeaveTripService struct {
	members   LeaveTripMemberRepository
	roleCache LeaveTripRoleCache
}

func NewLeaveTripService(members LeaveTripMemberRepository, roleCache LeaveTripRoleCache) *LeaveTripService {
	return &LeaveTripService{members: members, roleCache: roleCache}
}

// Leave marks the caller left. If they are the leader, leadership moves to the
// earliest-joined remaining member; the last leader must delete the trip instead.
func (s *LeaveTripService) Leave(ctx context.Context, userId, tripId uuid.UUID) error {
	leaver, err := s.members.FindMemberById(ctx, userId, tripId)
	if err != nil {
		return err
	}
	if !leaver.Status.IsActive() {
		return repository.ErrUserNotTripMember
	}

	var successor *model.TripMember
	if leaver.Role.IsLeader() {
		roster, err := s.members.ListActiveMembers(ctx, tripId)
		if err != nil {
			return err
		}
		successor = pickLeadershipSuccessor(roster, userId)
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
	_ = s.roleCache.DeleteCachedTripMemberRole(ctx, tripId, userId)
	if successor != nil {
		_ = s.roleCache.SetCachedTripMemberRole(ctx, tripId, successor.UserID, enum.TripRoleLeader)
	}
	return nil
}

func pickLeadershipSuccessor(members []model.TripMember, leavingUserID uuid.UUID) *model.TripMember {
	var earliest *model.TripMember
	for i := range members {
		m := &members[i]
		if m.UserID == leavingUserID || !m.Status.IsActive() || m.Role.IsLeader() {
			continue
		}
		if tripJoinedEarlier(m, earliest) {
			earliest = m
		}
	}
	return earliest
}

func tripJoinedEarlier(candidate, current *model.TripMember) bool {
	if current == nil {
		return true
	}
	return tripJoinedAtOrZero(candidate).Before(tripJoinedAtOrZero(current))
}

func tripJoinedAtOrZero(member *model.TripMember) time.Time {
	if member == nil || member.JoinedAt == nil {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return *member.JoinedAt
}
