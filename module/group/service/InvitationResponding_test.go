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

type stubInvitationMembers struct {
	member    *model.GroupMember
	listed    []model.GroupMember
	findErr   error
	updateErr error
	listErr   error
	updated   *model.GroupMember
}

func (s *stubInvitationMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.GroupMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotGroupMember
	}
	copied := *s.member
	if copied.Group != nil {
		groupCopy := *copied.Group
		copied.Group = &groupCopy
	}
	if copied.User != nil {
		userCopy := *copied.User
		copied.User = &userCopy
	}
	if copied.InvitorName != nil {
		name := *copied.InvitorName
		copied.InvitorName = &name
	}
	return &copied, nil
}

func (s *stubInvitationMembers) UpdateMember(_ context.Context, member *model.GroupMember) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	copied := *member
	s.updated = &copied
	s.member = &copied
	return nil
}

func (s *stubInvitationMembers) ListInvitedMembers(context.Context, uuid.UUID) ([]model.GroupMember, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listed, nil
}

type stubAcceptRoleCache struct {
	groupID uuid.UUID
	userID  uuid.UUID
	role    enum.GroupRole
	called  bool
}

func (s *stubAcceptRoleCache) SetCachedGroupMemberRole(_ context.Context, groupId, userId uuid.UUID, role enum.GroupRole) error {
	s.called = true
	s.groupID = groupId
	s.userID = userId
	s.role = role
	return nil
}

func invitedMember(groupID, userID uuid.UUID) *model.GroupMember {
	invitor := "alice"
	return &model.GroupMember{
		GroupID:     groupID,
		UserID:      userID,
		Group:       &model.Group{Name: "Ha Giang crew"},
		User:        &authModel.User{Username: "bob"},
		Role:        enum.GroupRoleMember,
		Status:      enum.MembershipInvited,
		InvitorName: &invitor,
	}
}

func TestAcceptInvitationActivatesMembership(t *testing.T) {
	groupID, userID := uuid.New(), uuid.New()
	members := &stubInvitationMembers{member: invitedMember(groupID, userID)}
	cache := &stubAcceptRoleCache{}
	svc := NewInvitationRespondingService(members, cache)

	got, err := svc.AcceptInvitation(context.Background(), userID, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipActive {
		t.Fatalf("status = %s, want active", got.Status)
	}
	if got.GroupName != "Ha Giang crew" || got.UserName != "bob" || got.InvitorName != "alice" {
		t.Fatalf("response = %+v", got)
	}
	if members.updated == nil || members.updated.JoinedAt == nil {
		t.Fatal("accept should set joinedAt")
	}
	if !cache.called || cache.role != enum.GroupRoleMember {
		t.Fatal("accept should cache the active member role")
	}
}

func TestAcceptInvitationMissingRowIsNotFound(t *testing.T) {
	svc := NewInvitationRespondingService(&stubInvitationMembers{}, nil)
	_, err := svc.AcceptInvitation(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrInvitationNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestAcceptInvitationRequiresInvitedStatus(t *testing.T) {
	member := invitedMember(uuid.New(), uuid.New())
	member.Status = enum.MembershipActive
	svc := NewInvitationRespondingService(&stubInvitationMembers{member: member}, nil)
	_, err := svc.AcceptInvitation(context.Background(), member.UserID, member.GroupID)
	if !errors.Is(err, repository.ErrInvitationNotPending) {
		t.Fatalf("got %v", err)
	}
}

func TestRejectInvitationSetsRejected(t *testing.T) {
	groupID, userID := uuid.New(), uuid.New()
	members := &stubInvitationMembers{member: invitedMember(groupID, userID)}
	svc := NewInvitationRespondingService(members, nil)

	got, err := svc.RejectInvitation(context.Background(), userID, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipRejected {
		t.Fatalf("status = %s, want rejected", got.Status)
	}
	if members.updated == nil || members.updated.JoinedAt != nil {
		t.Fatal("reject should clear joinedAt")
	}
}

func TestListInvitationsReturnsOnlyLoadedRows(t *testing.T) {
	member := invitedMember(uuid.New(), uuid.New())
	svc := NewInvitationRespondingService(&stubInvitationMembers{listed: []model.GroupMember{*member}}, nil)
	got, err := svc.ListInvitations(context.Background(), member.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Invitations) != 1 {
		t.Fatalf("len = %d", len(got.Invitations))
	}
	if got.Invitations[0].Status != enum.MembershipInvited {
		t.Fatalf("status = %s", got.Invitations[0].Status)
	}
}
