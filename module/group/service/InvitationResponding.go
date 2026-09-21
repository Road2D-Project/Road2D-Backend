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

type InvitationMemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	UpdateMember(ctx context.Context, member *model.GroupMember) error
	ListInvitedMembers(ctx context.Context, userId uuid.UUID) ([]model.GroupMember, error)
}

type InvitationRespondingService struct {
	members   InvitationMemberRepository
	roleCache GroupMemberRoleCache
}

func NewInvitationRespondingService(members InvitationMemberRepository, roleCache GroupMemberRoleCache) *InvitationRespondingService {
	return &InvitationRespondingService{members: members, roleCache: roleCache}
}

func (s *InvitationRespondingService) AcceptInvitation(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*response.InvitationResponse, error) {
	return s.respond(ctx, userId, groupId, enum.MembershipActive)
}

func (s *InvitationRespondingService) RejectInvitation(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*response.InvitationResponse, error) {
	return s.respond(ctx, userId, groupId, enum.MembershipRejected)
}

func (s *InvitationRespondingService) ListInvitations(ctx context.Context, userId uuid.UUID) (*response.InvitationListResponse, error) {
	members, err := s.members.ListInvitedMembers(ctx, userId)
	if err != nil {
		return nil, err
	}
	//map thủ công, không cần qua GORM
	items := make([]response.InvitationResponse, 0, len(members))
	for i := range members {
		items = append(items, response.FromGroupMember(&members[i]))
	}
	return &response.InvitationListResponse{Invitations: items}, nil
}

func (s *InvitationRespondingService) respond(ctx context.Context, userId, groupId uuid.UUID, status enum.MembershipStatus) (*response.InvitationResponse, error) {
	member, err := s.members.FindMemberById(ctx, userId, groupId)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotGroupMember) {
			return nil, repository.ErrInvitationNotFound
		}
		return nil, err
	}
	if !member.Status.IsInvited() {
		return nil, repository.ErrInvitationNotPending
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
