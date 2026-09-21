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

type stubJoinRequestMembers struct {
	member     *model.GroupMember
	listed     []model.GroupMember
	findErr    error
	updateErr  error
	requestErr error
	requested  *model.GroupMember
}

func (s *stubJoinRequestMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.GroupMember, error) {
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
	return &copied, nil
}

func (s *stubJoinRequestMembers) UpdateMember(_ context.Context, member *model.GroupMember) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	copied := *member
	s.member = &copied
	return nil
}

func (s *stubJoinRequestMembers) RequestJoin(_ context.Context, userId, groupId uuid.UUID) (*model.GroupMember, error) {
	if s.requestErr != nil {
		return nil, s.requestErr
	}
	s.requested = &model.GroupMember{
		GroupID: groupId,
		UserID:  userId,
		Group:   &model.Group{Name: "Crew"},
		User:    &authModel.User{Username: "bob"},
		Role:    enum.GroupRoleMember,
		Status:  enum.MembershipPending,
	}
	return s.requested, nil
}

func (s *stubJoinRequestMembers) ListPendingMembers(context.Context, uuid.UUID) ([]model.GroupMember, error) {
	return s.listed, nil
}

func pendingMember(groupID, userID uuid.UUID) *model.GroupMember {
	return &model.GroupMember{
		GroupID: groupID,
		UserID:  userID,
		Group:   &model.Group{Name: "Crew"},
		User:    &authModel.User{Username: "bob"},
		Role:    enum.GroupRoleMember,
		Status:  enum.MembershipPending,
	}
}

func TestRequestJoinCreatesPendingMembership(t *testing.T) {
	repo := &stubJoinRequestMembers{}
	svc := NewJoinRequestService(repo, nil)
	userID, groupID := uuid.New(), uuid.New()

	got, err := svc.RequestJoin(context.Background(), userID, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipPending {
		t.Fatalf("status = %s", got.Status)
	}
	if repo.requested == nil || repo.requested.UserID != userID {
		t.Fatal("should insert a pending membership")
	}
}

func TestRequestJoinActiveMemberConflicts(t *testing.T) {
	svc := NewJoinRequestService(&stubJoinRequestMembers{member: &model.GroupMember{Status: enum.MembershipActive}}, nil)
	_, err := svc.RequestJoin(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrAlreadyGroupMember) {
		t.Fatalf("got %v", err)
	}
}

func TestRequestJoinAlreadyInvitedConflicts(t *testing.T) {
	svc := NewJoinRequestService(&stubJoinRequestMembers{member: &model.GroupMember{Status: enum.MembershipInvited}}, nil)
	_, err := svc.RequestJoin(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrAlreadyInvited) {
		t.Fatalf("got %v", err)
	}
}

func TestRequestJoinReusesRejoinableRow(t *testing.T) {
	member := &model.GroupMember{
		Status: enum.MembershipLeft,
		Role:   enum.GroupRoleAdmin,
		Group:  &model.Group{Name: "Crew"},
		User:   &authModel.User{Username: "bob"},
	}
	repo := &stubJoinRequestMembers{member: member}
	svc := NewJoinRequestService(repo, nil)
	got, err := svc.RequestJoin(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipPending {
		t.Fatalf("status = %s", got.Status)
	}
	if repo.member.Role != enum.GroupRoleMember || repo.member.Status != enum.MembershipPending {
		t.Fatal("rejoin should reset role to member and status to pending")
	}
	if repo.requested != nil {
		t.Fatal("rejoin must not insert a new row")
	}
}

func TestAcceptJoinRequestActivatesMembership(t *testing.T) {
	groupID, userID := uuid.New(), uuid.New()
	members := &stubJoinRequestMembers{member: pendingMember(groupID, userID)}
	cache := &stubAcceptRoleCache{}
	svc := NewJoinRequestService(members, cache)

	got, err := svc.AcceptJoinRequest(context.Background(), userID, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipActive {
		t.Fatalf("status = %s", got.Status)
	}
	if members.member.JoinedAt == nil {
		t.Fatal("accept should set joinedAt")
	}
	if !cache.called || cache.role != enum.GroupRoleMember {
		t.Fatal("accept should cache the active member role")
	}
}

func TestRejectJoinRequestSetsRejected(t *testing.T) {
	groupID, userID := uuid.New(), uuid.New()
	members := &stubJoinRequestMembers{member: pendingMember(groupID, userID)}
	svc := NewJoinRequestService(members, nil)

	got, err := svc.RejectJoinRequest(context.Background(), userID, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipRejected {
		t.Fatalf("status = %s", got.Status)
	}
	if members.member.JoinedAt != nil {
		t.Fatal("reject should clear joinedAt")
	}
}

func TestAcceptJoinRequestMissingRowIsNotFound(t *testing.T) {
	svc := NewJoinRequestService(&stubJoinRequestMembers{}, nil)
	_, err := svc.AcceptJoinRequest(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrJoinRequestNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestAcceptJoinRequestRequiresPendingStatus(t *testing.T) {
	member := pendingMember(uuid.New(), uuid.New())
	member.Status = enum.MembershipInvited
	svc := NewJoinRequestService(&stubJoinRequestMembers{member: member}, nil)
	_, err := svc.AcceptJoinRequest(context.Background(), member.UserID, member.GroupID)
	if !errors.Is(err, repository.ErrJoinRequestNotPending) {
		t.Fatalf("got %v", err)
	}
}

func TestListJoinRequestsReturnsPendingRows(t *testing.T) {
	member := pendingMember(uuid.New(), uuid.New())
	svc := NewJoinRequestService(&stubJoinRequestMembers{listed: []model.GroupMember{*member}}, nil)
	got, err := svc.ListJoinRequests(context.Background(), member.GroupID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.JoinRequests) != 1 {
		t.Fatalf("len = %d", len(got.JoinRequests))
	}
	if got.JoinRequests[0].Status != enum.MembershipPending {
		t.Fatalf("status = %s", got.JoinRequests[0].Status)
	}
}
