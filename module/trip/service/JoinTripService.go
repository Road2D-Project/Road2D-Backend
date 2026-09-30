package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"

	"github.com/google/uuid"
)

type JoinTripFinder interface {
	FindTripByInviteToken(ctx context.Context, token string) (*model.Trip, error)
}

type JoinTripMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
	AddMember(ctx context.Context, userId uuid.UUID, tripId uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, invitorName *string) (*model.TripMember, error)
	CountActiveMembers(ctx context.Context, tripId uuid.UUID) (int, error)
}

type JoinTripService struct {
	trips   JoinTripFinder
	members JoinTripMemberRepository
}

func NewJoinTripService(trips JoinTripFinder, members JoinTripMemberRepository) *JoinTripService {
	return &JoinTripService{trips: trips, members: members}
}

// Join records a pending request for the trip behind the invite token.
// A leader or admin still has to approve it before the caller has a seat.
func (s *JoinTripService) Join(ctx context.Context, user *authModel.User, token string) (*response.TripMemberResponse, error) {
	if user == nil {
		return nil, errors.New("missing current user")
	}
	trip, err := s.trips.FindTripByInviteToken(ctx, token)
	if err != nil {
		return nil, err
	}
	active, err := s.members.CountActiveMembers(ctx, trip.ID)
	if err != nil {
		return nil, err
	}
	if err := seatsFit(active+1, trip.MemberLimit); err != nil {
		return nil, err
	}
	member, err := s.members.FindMemberById(ctx, user.ID, trip.ID)
	if err != nil && !errors.Is(err, repository.ErrUserNotTripMember) {
		return nil, err
	}
	if member != nil {
		updated, err := s.reuseMembershipForJoin(ctx, member)
		if err != nil {
			return nil, err
		}
		return response.FromTripMemberPtr(updated), nil
	}
	created, err := s.members.AddMember(ctx, user.ID, trip.ID, enum.TripRoleMember, enum.MembershipPending, user.Username, nil)
	if err != nil {
		return nil, err
	}
	return response.FromTripMemberPtr(created), nil
}

func (s *JoinTripService) reuseMembershipForJoin(ctx context.Context, member *model.TripMember) (*model.TripMember, error) {
	switch {
	case member.Status.IsActive():
		return nil, repository.ErrAlreadyTripMember
	case member.Status.IsInvited():
		return nil, repository.ErrAlreadyInvited
	case member.Status.IsPending():
		return nil, repository.ErrJoinRequestPending
	case member.Status.CanRejoin():
		member.InvitorName = nil
		member.Role = enum.TripRoleMember
		member.Status = enum.MembershipPending
		member.JoinedAt = nil
		if err := s.members.UpdateMember(ctx, member); err != nil {
			return nil, err
		}
		return member, nil
	default:
		return nil, repository.ErrAlreadyTripMember
	}
}
