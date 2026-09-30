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

type TripJoinRequestFinder interface {
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
}

type TripJoinRequestStore interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
	ListPendingMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error)
	CountActiveMembers(ctx context.Context, tripId uuid.UUID) (int, error)
}

type TripJoinRequestRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

type TripJoinRequestService struct {
	trips     TripJoinRequestFinder
	members   TripJoinRequestStore
	roleCache TripJoinRequestRoleCache
}

func NewTripJoinRequestService(trips TripJoinRequestFinder, members TripJoinRequestStore, roleCache TripJoinRequestRoleCache) *TripJoinRequestService {
	return &TripJoinRequestService{trips: trips, members: members, roleCache: roleCache}
}

func (s *TripJoinRequestService) List(ctx context.Context, tripID uuid.UUID) (*response.TripJoinRequestListResponse, error) {
	rows, err := s.members.ListPendingMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}
	items := make([]response.TripMemberResponse, 0, len(rows))
	for i := range rows {
		items = append(items, response.FromTripMember(&rows[i]))
	}
	return &response.TripJoinRequestListResponse{JoinRequests: items}, nil
}

func (s *TripJoinRequestService) Accept(ctx context.Context, userID, tripID uuid.UUID) (*response.TripMemberResponse, error) {
	return s.respond(ctx, userID, tripID, enum.MembershipActive)
}

func (s *TripJoinRequestService) Reject(ctx context.Context, userID, tripID uuid.UUID) (*response.TripMemberResponse, error) {
	return s.respond(ctx, userID, tripID, enum.MembershipRejected)
}

func (s *TripJoinRequestService) respond(ctx context.Context, userID, tripID uuid.UUID, status enum.MembershipStatus) (*response.TripMemberResponse, error) {
	member, err := s.members.FindMemberById(ctx, userID, tripID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotTripMember) {
			return nil, repository.ErrJoinRequestNotFound
		}
		return nil, err
	}
	if !member.Status.IsPending() {
		return nil, repository.ErrJoinRequestNotPending
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
		member.Role = enum.TripRoleMember
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
