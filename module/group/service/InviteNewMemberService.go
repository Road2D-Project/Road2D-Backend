package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"

	"github.com/google/uuid"
)

type MemberRepository interface {
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	UpdateMember(ctx context.Context, member *model.GroupMember) error
	InviteMember(ctx context.Context, userId uuid.UUID, groupId uuid.UUID, invitorName string) (*model.GroupMember, error)
}

type InviteNewMemberService struct {
	memberRepository MemberRepository
}

func NewInviteNewMemberService(memberRepository MemberRepository) *InviteNewMemberService {
	return &InviteNewMemberService{memberRepository: memberRepository}
}

func (s *InviteNewMemberService) Invite(ctx context.Context, invitedUserId, groupId, invitorID uuid.UUID, invitorName string) (*response.InvitationResponse, error) {
	if invitedUserId == invitorID {
		return nil, repository.ErrCannotInviteSelf
	}
	member, err := s.memberRepository.FindMemberById(ctx, invitedUserId, groupId)
	if err != nil && !errors.Is(err, repository.ErrUserNotGroupMember) {
		return nil, err
	}
	if member != nil {
		updated, err := s.reuseMembershipForInvite(ctx, member, invitorName)
		if err != nil {
			return nil, err
		}
		return response.FromGroupMemberPtr(updated), nil
	}
	created, err := s.memberRepository.InviteMember(ctx, invitedUserId, groupId, invitorName)
	if err != nil {
		return nil, err
	}
	return response.FromGroupMemberPtr(created), nil
}

func (s *InviteNewMemberService) reuseMembershipForInvite(ctx context.Context, member *model.GroupMember, invitorName string) (*model.GroupMember, error) {
	switch {
	case member.Status.IsActive():
		return nil, repository.ErrAlreadyGroupMember
	case member.Status.IsInvited():
		return nil, repository.ErrAlreadyInvited
	case member.Status.IsPending():
		return nil, repository.ErrJoinRequestPending
	case member.Status.CanRejoin():
		member.InvitorName = &invitorName
		member.Role = enum.GroupRoleMember
		member.Status = enum.MembershipInvited
		member.JoinedAt = nil
		if err := s.memberRepository.UpdateMember(ctx, member); err != nil {
			return nil, err
		}
		return member, nil
	default:
		return nil, repository.ErrAlreadyGroupMember
	}
}
