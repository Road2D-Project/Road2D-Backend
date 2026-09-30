package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
)

type TripMemberRecords interface {
	ListActiveMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error)
	FindMemberById(ctx context.Context, userId uuid.UUID, tripId uuid.UUID) (*model.TripMember, error)
	UpdateMember(ctx context.Context, member *model.TripMember) error
}

type TripMemberRoleWriter interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

type TripMemberService struct {
	members   TripMemberRecords
	roleCache TripMemberRoleWriter
}

func NewTripMemberService(members TripMemberRecords, roleCache TripMemberRoleWriter) *TripMemberService {
	return &TripMemberService{members: members, roleCache: roleCache}
}

func (s *TripMemberService) ListActiveMembers(ctx context.Context, tripID uuid.UUID) (*response.TripMemberListResponse, error) {
	rows, err := s.members.ListActiveMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return tripRosterRank(rows[i].Role) < tripRosterRank(rows[j].Role)
	})
	items := make([]response.TripMemberResponse, 0, len(rows))
	for i := range rows {
		items = append(items, response.FromTripMember(&rows[i]))
	}
	return &response.TripMemberListResponse{Members: items}, nil
}

func (s *TripMemberService) Update(ctx context.Context, actorID, targetID, tripID uuid.UUID, actorRole enum.TripRole, req request.UpdateTripMemberRequest) (*response.TripMemberResponse, error) {
	if req.Nickname == nil && req.Role == nil {
		return nil, repository.ErrNoTripMemberUpdate
	}
	member, err := s.members.FindMemberById(ctx, targetID, tripID)
	if err != nil {
		return nil, err
	}
	if !member.Status.IsActive() {
		return nil, repository.ErrUserNotTripMember
	}
	if req.Nickname != nil {
		if actorID != targetID {
			return nil, repository.ErrCannotUpdateOtherNickname
		}
		cleaned := utils.Santize(*req.Nickname)
		if cleaned == "" {
			return nil, errors.New("nickname is required")
		}
		member.Nickname = cleaned
	}
	if req.Role != nil {
		if !actorRole.CanChangeRoles() {
			return nil, repository.ErrInsufficientKickRole
		}
		if member.Role.IsLeader() || req.Role.IsLeader() {
			return nil, repository.ErrCannotChangeLeaderRole
		}
		if *req.Role != enum.TripRoleAdmin && *req.Role != enum.TripRoleMember {
			return nil, repository.ErrCannotChangeLeaderRole
		}
		member.Role = *req.Role
	}
	if err := s.members.UpdateMember(ctx, member); err != nil {
		return nil, err
	}
	if req.Role != nil && s.roleCache != nil {
		_ = s.roleCache.SetCachedTripMemberRole(ctx, tripID, targetID, member.Role)
	}
	out := response.FromTripMember(member)
	return &out, nil
}

func tripRosterRank(role enum.TripRole) int {
	switch {
	case role.IsLeader():
		return 0
	case role.IsAdmin():
		return 1
	default:
		return 2
	}
}
