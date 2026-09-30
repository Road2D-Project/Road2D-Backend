package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"

	"github.com/google/uuid"
)

type InviteTripFinder interface {
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type InviteTripStore interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
	AddMember(ctx context.Context, userId uuid.UUID, tripId uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, invitorName *string) (*model.TripMember, error)
	CountActiveMembers(ctx context.Context, tripId uuid.UUID) (int, error)
}

type InviteTripMemberService struct {
	trips   InviteTripFinder
	members InviteTripStore
}

func NewInviteTripMemberService(trips InviteTripFinder, members InviteTripStore) *InviteTripMemberService {
	return &InviteTripMemberService{trips: trips, members: members}
}

// Invite lets any active member name a user id. A leader or admin invite waits
// for the invitee to accept. A member invite stays pending until a leader or
// admin approves it. Friend checks are not applied yet.
func (s *InviteTripMemberService) Invite(ctx context.Context, invitedUserID, tripID, invitorID uuid.UUID, invitorName string, invitorRole enum.TripRole) (*response.TripMemberResponse, error) {
	if invitedUserID == invitorID {
		return nil, repository.ErrCannotInviteSelf
	}
	trip, err := s.trips.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	active, err := s.members.CountActiveMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if err := seatsFit(active+1, trip.MemberLimit); err != nil {
		return nil, err
	}
	status := enum.MembershipPending
	if invitorRole.CanManageMembers() {
		status = enum.MembershipInvited
	}
	member, err := s.members.FindMemberById(ctx, invitedUserID, tripID)
	if err != nil && !errors.Is(err, repository.ErrUserNotTripMember) {
		return nil, err
	}
	if member != nil {
		updated, err := s.reuseMembershipForInvite(ctx, member, invitorName, status)
		if err != nil {
			return nil, err
		}
		return response.FromTripMemberPtr(updated), nil
	}
	created, err := s.members.AddMember(ctx, invitedUserID, tripID, enum.TripRoleMember, status, "", &invitorName)
	if err != nil {
		return nil, err
	}
	return response.FromTripMemberPtr(created), nil
}

func (s *InviteTripMemberService) reuseMembershipForInvite(ctx context.Context, member *model.TripMember, invitorName string, status enum.MembershipStatus) (*model.TripMember, error) {
	switch {
	case member.Status.IsActive():
		return nil, repository.ErrAlreadyTripMember
	case member.Status.IsInvited():
		return nil, repository.ErrAlreadyInvited
	case member.Status.IsPending():
		return nil, repository.ErrJoinRequestPending
	case member.Status.CanRejoin():
		member.InvitorName = &invitorName
		member.Role = enum.TripRoleMember
		member.Status = status
		member.JoinedAt = nil
		if err := s.members.UpdateMember(ctx, member); err != nil {
			return nil, err
		}
		return member, nil
	default:
		return nil, repository.ErrAlreadyTripMember
	}
}
