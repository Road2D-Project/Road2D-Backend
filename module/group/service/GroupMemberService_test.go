package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubActiveMembers struct {
	members []model.GroupMember
	err     error
}

func (s *stubActiveMembers) ListActiveMembers(context.Context, uuid.UUID) ([]model.GroupMember, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.members, nil
}

func TestListActiveMembersOrdersOwnerAdminThenMember(t *testing.T) {
	now := time.Now().UTC()
	svc := NewGroupMemberService(&stubActiveMembers{members: []model.GroupMember{
		{
			UserID:   uuid.New(),
			Nickname: "scout",
			Role:     enum.GroupRoleMember,
			Status:   enum.MembershipActive,
			User:     &authModel.User{Username: "scout"},
			JoinedAt: &now,
		},
		{
			UserID:   uuid.New(),
			Nickname: "leader",
			Role:     enum.GroupRoleOwner,
			Status:   enum.MembershipActive,
			User:     &authModel.User{Username: "leader"},
			JoinedAt: &now,
		},
		{
			UserID:   uuid.New(),
			Nickname: "guide",
			Role:     enum.GroupRoleAdmin,
			Status:   enum.MembershipActive,
			User:     &authModel.User{Username: "guide"},
			JoinedAt: &now,
		},
	}})

	got, err := svc.ListActiveMembers(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Members) != 3 {
		t.Fatalf("len = %d", len(got.Members))
	}
	if got.Members[0].Role != enum.GroupRoleOwner || got.Members[0].UserName != "leader" {
		t.Fatalf("first = %+v", got.Members[0])
	}
	if got.Members[1].Role != enum.GroupRoleAdmin || got.Members[2].Role != enum.GroupRoleMember {
		t.Fatalf("roster = %+v", got.Members)
	}
}

func TestListActiveMembersEmptyRoster(t *testing.T) {
	svc := NewGroupMemberService(&stubActiveMembers{})
	got, err := svc.ListActiveMembers(context.Background(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got.Members == nil || len(got.Members) != 0 {
		t.Fatalf("members = %#v", got.Members)
	}
}
