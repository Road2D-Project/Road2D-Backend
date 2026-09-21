package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"sort"

	"github.com/google/uuid"
)

type ActiveMemberLister interface {
	ListActiveMembers(ctx context.Context, groupId uuid.UUID) ([]model.GroupMember, error)
}

type GroupMemberService struct {
	members ActiveMemberLister
}

func NewGroupMemberService(members ActiveMemberLister) *GroupMemberService {
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
