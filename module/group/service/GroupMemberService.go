package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"sort"

	"github.com/google/uuid"
)

type GroupMemberRecords interface {
	ListActiveMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error)
	FindMemberById(ctx context.Context, userId uuid.UUID, groupId uuid.UUID) (*model.GroupMember, error)
	UpdateMember(ctx context.Context, member *model.GroupMember) error
}

type GroupMemberService struct {
	members GroupMemberRecords
}

func NewGroupMemberService(members GroupMemberRecords) *GroupMemberService {
	return &GroupMemberService{members: members}
}

func (s *GroupMemberService) ListActiveMembers(ctx context.Context, groupId uuid.UUID) (*response.GroupMemberListResponse, error) {
	rows, err := s.members.ListActiveMembers(ctx, groupId)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rosterRank(rows[i].Role) < rosterRank(rows[j].Role)
	})
	items := make([]response.GroupMemberResponse, 0, len(rows))
	for i := range rows {
		items = append(items, response.MemberFromGroupMember(&rows[i]))
	}
	return &response.GroupMemberListResponse{Members: items}, nil
}

func (s *GroupMemberService) UpdateNickname(ctx context.Context, callerID, targetID, groupID uuid.UUID, nickname string) (*response.GroupMemberResponse, error) {
	if callerID != targetID {
		return nil, repository.ErrCannotUpdateOtherNickname
	}
	cleaned := utils.Santize(nickname)
	if cleaned == "" {
		return nil, errors.New("nickname is required")
	}
	member, err := s.members.FindMemberById(ctx, targetID, groupID)
	if err != nil {
		return nil, err
	}
	if !member.Status.IsActive() {
		return nil, repository.ErrUserNotGroupMember
	}
	member.Nickname = cleaned
	if err := s.members.UpdateMember(ctx, member); err != nil {
		return nil, err
	}
	out := response.MemberFromGroupMember(member)
	return &out, nil
}

func rosterRank(role enum.GroupRole) int {
	switch {
	case role.IsOwner():
		return 0
	case role.IsAdmin():
		return 1
	default:
		return 2
	}
}
