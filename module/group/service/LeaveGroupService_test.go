package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubLeaveMembers struct {
	member    *model.GroupMember
	roster    []model.GroupMember
	findErr   error
	leaveErr  error
	leaver    *model.GroupMember
	successor *model.GroupMember
}

func (s *stubLeaveMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.GroupMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotGroupMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubLeaveMembers) ListActiveMembers(context.Context, uuid.UUID) ([]model.GroupMember, error) {
	return s.roster, nil
}

func (s *stubLeaveMembers) ApplyLeave(_ context.Context, leaver *model.GroupMember, successor *model.GroupMember) error {
	if s.leaveErr != nil {
		return s.leaveErr
	}
	copied := *leaver
	s.leaver = &copied
	if successor != nil {
		succ := *successor
		s.successor = &succ
	}
	return nil
}

type stubLeaveCache struct {
	deletedUser uuid.UUID
	setUser     uuid.UUID
	setRole     enum.GroupRole
	deleted     bool
}

func (s *stubLeaveCache) SetCachedGroupMemberRole(_ context.Context, _, userId uuid.UUID, role enum.GroupRole) error {
	s.setUser = userId
	s.setRole = role
	return nil
}

func (s *stubLeaveCache) DeleteCachedGroupMemberRole(_ context.Context, _, userId uuid.UUID) error {
	s.deleted = true
	s.deletedUser = userId
	return nil
}

func at(hours int) *time.Time {
	t := time.Date(2026, 9, 21, hours, 0, 0, 0, time.UTC)
	return &t
}

func TestLeaveMarksActiveMemberLeft(t *testing.T) {
	userID, groupID := uuid.New(), uuid.New()
	members := &stubLeaveMembers{member: &model.GroupMember{
		UserID: userID, GroupID: groupID, Role: enum.GroupRoleMember, Status: enum.MembershipActive,
	}}
	cache := &stubLeaveCache{}
	svc := NewLeaveGroupService(members, cache)

	if err := svc.Leave(context.Background(), userID, groupID); err != nil {
		t.Fatal(err)
	}
	if members.successor != nil {
		t.Fatal("member leave must not transfer ownership")
	}
	if !cache.deleted || cache.deletedUser != userID {
		t.Fatal("leave should drop the caller's cached role")
	}
}

func TestLeaveOwnerTransfersToEarliestAdmin(t *testing.T) {
	ownerID, laterAdminID, earlyAdminID, memberID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	groupID := uuid.New()
	members := &stubLeaveMembers{
		member: &model.GroupMember{UserID: ownerID, GroupID: groupID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive},
		roster: []model.GroupMember{
			{UserID: ownerID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive, JoinedAt: at(1)},
			{UserID: laterAdminID, Role: enum.GroupRoleAdmin, Status: enum.MembershipActive, JoinedAt: at(4)},
			{UserID: memberID, Role: enum.GroupRoleMember, Status: enum.MembershipActive, JoinedAt: at(2)},
			{UserID: earlyAdminID, Role: enum.GroupRoleAdmin, Status: enum.MembershipActive, JoinedAt: at(3)},
		},
	}
	cache := &stubLeaveCache{}
	svc := NewLeaveGroupService(members, cache)

	if err := svc.Leave(context.Background(), ownerID, groupID); err != nil {
		t.Fatal(err)
	}
	if members.successor == nil || members.successor.UserID != earlyAdminID {
		t.Fatalf("successor = %+v, want earliest admin", members.successor)
	}
	if cache.setUser != earlyAdminID || cache.setRole != enum.GroupRoleOwner {
		t.Fatal("new owner role should be cached")
	}
}

func TestLeaveOwnerTransfersToEarliestMemberWhenNoAdmin(t *testing.T) {
	ownerID, lateMemberID, earlyMemberID := uuid.New(), uuid.New(), uuid.New()
	groupID := uuid.New()
	members := &stubLeaveMembers{
		member: &model.GroupMember{UserID: ownerID, GroupID: groupID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive},
		roster: []model.GroupMember{
			{UserID: lateMemberID, Role: enum.GroupRoleMember, Status: enum.MembershipActive, JoinedAt: at(9)},
			{UserID: ownerID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive, JoinedAt: at(1)},
			{UserID: earlyMemberID, Role: enum.GroupRoleMember, Status: enum.MembershipActive, JoinedAt: at(5)},
		},
	}
	svc := NewLeaveGroupService(members, nil)

	if err := svc.Leave(context.Background(), ownerID, groupID); err != nil {
		t.Fatal(err)
	}
	if members.successor == nil || members.successor.UserID != earlyMemberID {
		t.Fatalf("successor = %+v, want earliest member", members.successor)
	}
}

func TestLeaveOwnerAloneHasNoSuccessor(t *testing.T) {
	ownerID, groupID := uuid.New(), uuid.New()
	svc := NewLeaveGroupService(&stubLeaveMembers{
		member: &model.GroupMember{UserID: ownerID, GroupID: groupID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive},
		roster: []model.GroupMember{
			{UserID: ownerID, Role: enum.GroupRoleOwner, Status: enum.MembershipActive, JoinedAt: at(1)},
		},
	}, nil)
	err := svc.Leave(context.Background(), ownerID, groupID)
	if !errors.Is(err, repository.ErrNoSuccessorToTransfer) {
		t.Fatalf("got %v", err)
	}
}

func TestLeaveInactiveMemberIsForbidden(t *testing.T) {
	svc := NewLeaveGroupService(&stubLeaveMembers{member: &model.GroupMember{
		Status: enum.MembershipInvited, Role: enum.GroupRoleMember,
	}}, nil)
	err := svc.Leave(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrUserNotGroupMember) {
		t.Fatalf("got %v", err)
	}
}
