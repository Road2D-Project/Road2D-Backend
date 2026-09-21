package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type JoinTripFinder interface {
	FindTripByInviteToken(ctx context.Context, token string) (*model.Trip, error)
}

type JoinTripMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
	AddActiveMember(ctx context.Context, userId uuid.UUID, tripId uuid.UUID, role enum.TripRole, nickname string) (*model.TripMember, error)
}

type JoinTripRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

type JoinTripService struct {
	trips     JoinTripFinder
	members   JoinTripMemberRepository
	roleCache JoinTripRoleCache
}

func NewJoinTripService(trips JoinTripFinder, members JoinTripMemberRepository, roleCache JoinTripRoleCache) *JoinTripService {
	return &JoinTripService{trips: trips, members: members, roleCache: roleCache}
}

// Join resolves the trip from the invite token, then either inserts an active
// member row or reactivates a left/rejected/kicked row. Already-active seats fail.
func (s *JoinTripService) Join(ctx context.Context, user *authModel.User, token string) (*response.TripResponse, error) {
	if user == nil {
		return nil, errors.New("missing current user")
	}
	trip, err := s.trips.FindTripByInviteToken(ctx, token)
	if err != nil {
		return nil, err
	}
	member, err := s.members.FindMemberById(ctx, user.ID, trip.ID)
	if err != nil && !errors.Is(err, repository.ErrUserNotTripMember) {
		return nil, err
	}
	if member != nil {
		if err := s.reactivateMembership(ctx, member, user.Username); err != nil {
			return nil, err
		}
	} else {
		if _, err := s.members.AddActiveMember(ctx, user.ID, trip.ID, enum.TripRoleMember, user.Username); err != nil {
			return nil, err
		}
	}
	if s.roleCache != nil {
		_ = s.roleCache.SetCachedTripMemberRole(ctx, trip.ID, user.ID, enum.TripRoleMember)
	}
	return response.FromTripPtr(trip, response.RolePtr(enum.TripRoleMember)), nil
}

func (s *JoinTripService) reactivateMembership(ctx context.Context, member *model.TripMember, nickname string) error {
	if member.Status.IsActive() {
		return repository.ErrAlreadyTripMember
	}
	if !member.Status.CanRejoin() && !member.Status.IsInvited() && !member.Status.IsPending() {
		return repository.ErrAlreadyTripMember
	}
	now := time.Now().UTC()
	member.Role = enum.TripRoleMember
	member.Status = enum.MembershipActive
	member.Nickname = nickname
	member.JoinedAt = &now
	return s.members.UpdateMember(ctx, member)
}
