package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubInviteMembers struct {
	member     *model.GroupMember
	findErr    error
	updateErr  error
	inviteErr  error
	invited    *model.GroupMember
	inviteUser uuid.UUID
	inviteGrp  uuid.UUID
}

func (s *stubInviteMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.GroupMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotGroupMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubInviteMembers) UpdateMember(_ context.Context, member *model.GroupMember) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	copied := *member
	s.member = &copied
	return nil
}

func (s *stubInviteMembers) InviteMember(_ context.Context, userId, groupId uuid.UUID, invitorName string) (*model.GroupMember, error) {
	if s.inviteErr != nil {
		return nil, s.inviteErr
	}
	s.inviteUser = userId
	s.inviteGrp = groupId
	name := invitorName
	s.invited = &model.GroupMember{
		GroupID:     groupId,
		UserID:      userId,
		Group:       &model.Group{Name: "Crew"},
		User:        &authModel.User{Username: "bob"},
		Role:        enum.GroupRoleMember,
		Status:      enum.MembershipInvited,
		InvitorName: &name,
	}
	return s.invited, nil
}

func TestInviteCreatesNewMembership(t *testing.T) {
	repo := &stubInviteMembers{}
	svc := NewInviteNewMemberService(repo)
	invitorID, invitedID, groupID := uuid.New(), uuid.New(), uuid.New()

	got, err := svc.Invite(context.Background(), invitedID, groupID, invitorID, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipInvited || got.InvitorName != "alice" {
		t.Fatalf("got %+v", got)
	}
	if repo.inviteUser != invitedID || repo.inviteGrp != groupID {
		t.Fatal("invite should persist invited user into the group")
	}
}

func TestInviteRejectsSelf(t *testing.T) {
	id := uuid.New()
	svc := NewInviteNewMemberService(&stubInviteMembers{})
	_, err := svc.Invite(context.Background(), id, uuid.New(), id, "alice")
	if !errors.Is(err, repository.ErrCannotInviteSelf) {
		t.Fatalf("got %v", err)
	}
}

func TestInviteActiveMemberConflicts(t *testing.T) {
	member := &model.GroupMember{Status: enum.MembershipActive}
	svc := NewInviteNewMemberService(&stubInviteMembers{member: member})
	_, err := svc.Invite(context.Background(), uuid.New(), uuid.New(), uuid.New(), "alice")
	if !errors.Is(err, repository.ErrAlreadyGroupMember) {
		t.Fatalf("got %v", err)
	}
}

func TestInviteAlreadyInvitedConflicts(t *testing.T) {
	member := &model.GroupMember{Status: enum.MembershipInvited}
	svc := NewInviteNewMemberService(&stubInviteMembers{member: member})
	_, err := svc.Invite(context.Background(), uuid.New(), uuid.New(), uuid.New(), "alice")
	if !errors.Is(err, repository.ErrAlreadyInvited) {
		t.Fatalf("got %v", err)
	}
}

func TestInviteReusesRejoinableRow(t *testing.T) {
	member := &model.GroupMember{
		Status: enum.MembershipLeft,
		Role:   enum.GroupRoleAdmin,
		Group:  &model.Group{Name: "Crew"},
		User:   &authModel.User{Username: "bob"},
	}
	repo := &stubInviteMembers{member: member}
	svc := NewInviteNewMemberService(repo)
	got, err := svc.Invite(context.Background(), uuid.New(), uuid.New(), uuid.New(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipInvited {
		t.Fatalf("status = %s", got.Status)
	}
	if repo.member.Role != enum.GroupRoleMember || repo.member.Status != enum.MembershipInvited {
		t.Fatal("rejoin should reset role to member and status to invited")
	}
	if repo.invited != nil {
		t.Fatal("rejoin must not insert a new row")
	}
}
