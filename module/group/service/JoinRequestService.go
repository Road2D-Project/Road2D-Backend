package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

type JoinRequestRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	UpdateMember(ctx context.Context, member *model.GroupMember) error
	RequestJoin(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	ListPendingMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error)
}

type JoinRequestService struct {
	members   JoinRequestRepository
	roleCache GroupMemberRoleCache
}

func NewJoinRequestService(members JoinRequestRepository, roleCache GroupMemberRoleCache) *JoinRequestService {
	return &JoinRequestService{members: members, roleCache: roleCache}
}

func (s *JoinRequestService) RequestJoin(ctx context.Context, userId, groupId uuid.UUID) (*response.InvitationResponse, error) {
	member, err := s.members.FindMemberById(ctx, userId, groupId)
	if err != nil && !errors.Is(err, repository.ErrUserNotGroupMember) {
		return nil, err
	}
	if member != nil {
		updated, err := s.reuseMembershipForJoin(ctx, member)
		if err != nil {
			return nil, err
		}
		return response.FromGroupMemberPtr(updated), nil
	}
	created, err := s.members.RequestJoin(ctx, userId, groupId)
	if err != nil {
		return nil, err
	}
	return response.FromGroupMemberPtr(created), nil
}

func (s *JoinRequestService) ListJoinRequests(ctx context.Context, groupId uuid.UUID) (*response.JoinRequestListResponse, error) {
	members, err := s.members.ListPendingMembers(ctx, groupId)
	if err != nil {
		return nil, err
	}
	items := make([]response.InvitationResponse, 0, len(members))
	for i := range members {
		items = append(items, response.FromGroupMember(&members[i]))
	}
	return &response.JoinRequestListResponse{JoinRequests: items}, nil
}

func (s *JoinRequestService) AcceptJoinRequest(ctx context.Context, userId, groupId uuid.UUID) (*response.InvitationResponse, error) {
	return s.respond(ctx, userId, groupId, enum.MembershipActive)
}

func (s *JoinRequestService) RejectJoinRequest(ctx context.Context, userId, groupId uuid.UUID) (*response.InvitationResponse, error) {
	return s.respond(ctx, userId, groupId, enum.MembershipRejected)
}

func (s *JoinRequestService) reuseMembershipForJoin(ctx context.Context, member *model.GroupMember) (*model.GroupMember, error) {
	switch {
	case member.Status.IsActive():
		return nil, repository.ErrAlreadyGroupMember
	case member.Status.IsInvited():
		return nil, repository.ErrAlreadyInvited
	case member.Status.IsPending():
		return nil, repository.ErrJoinRequestPending
	case member.Status.CanRejoin():
		member.InvitorName = nil
		member.Role = enum.GroupRoleMember
		member.Status = enum.MembershipPending
		member.JoinedAt = nil
		if err := s.members.UpdateMember(ctx, member); err != nil {
			return nil, err
		}
		return member, nil
	default:
		return nil, repository.ErrAlreadyGroupMember
	}
}

func (s *JoinRequestService) respond(ctx context.Context, userId, groupId uuid.UUID, status enum.MembershipStatus) (*response.InvitationResponse, error) {
	member, err := s.members.FindMemberById(ctx, userId, groupId)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotGroupMember) {
			return nil, repository.ErrJoinRequestNotFound
		}
		return nil, err
	}
	if !member.Status.IsPending() {
		return nil, repository.ErrJoinRequestNotPending
	}
	if status.IsActive() {
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
		_ = s.roleCache.SetCachedGroupMemberRole(ctx, member.GroupID, member.UserID, member.Role)
	}
	return response.FromGroupMemberPtr(member), nil
}
