package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type TripInvitationFinder interface {
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type TripInvitationStore interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
	ListInvitedMembers(ctx context.Context, userId uuid.UUID) ([]model.TripMember, error)
	CountActiveMembers(ctx context.Context, tripId uuid.UUID) (int, error)
}

type TripInvitationRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

type TripInvitationService struct {
	trips     TripInvitationFinder
	members   TripInvitationStore
	roleCache TripInvitationRoleCache
}

func NewTripInvitationService(trips TripInvitationFinder, members TripInvitationStore, roleCache TripInvitationRoleCache) *TripInvitationService {
	return &TripInvitationService{trips: trips, members: members, roleCache: roleCache}
}

func (s *TripInvitationService) List(ctx context.Context, userID uuid.UUID) (*response.TripInvitationListResponse, error) {
	rows, err := s.members.ListInvitedMembers(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]response.TripMemberResponse, 0, len(rows))
	for i := range rows {
		items = append(items, response.FromTripMember(&rows[i]))
	}
	return &response.TripInvitationListResponse{Invitations: items}, nil
}

func (s *TripInvitationService) Accept(ctx context.Context, userID, tripID uuid.UUID) (*response.TripMemberResponse, error) {
	return s.respond(ctx, userID, tripID, enum.MembershipActive)
}

func (s *TripInvitationService) Reject(ctx context.Context, userID, tripID uuid.UUID) (*response.TripMemberResponse, error) {
	return s.respond(ctx, userID, tripID, enum.MembershipRejected)
}

func (s *TripInvitationService) respond(ctx context.Context, userID, tripID uuid.UUID, status enum.MembershipStatus) (*response.TripMemberResponse, error) {
	member, err := s.members.FindMemberById(ctx, userID, tripID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotTripMember) {
			return nil, repository.ErrInvitationNotFound
		}
		return nil, err
	}
	if !member.Status.IsInvited() {
		return nil, repository.ErrInvitationNotPending
	}
	if status.IsActive() {
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
		now := time.Now().UTC()
		member.JoinedAt = &now
	} else {
		member.JoinedAt = nil
	}
	member.Status = status
	if err := s.members.UpdateMember(ctx, member); err != nil {
		return nil, err
	}
	if status.IsActive() && s.roleCache != nil {
		_ = s.roleCache.SetCachedTripMemberRole(ctx, tripID, userID, member.Role)
	}
	return response.FromTripMemberPtr(member), nil
}
