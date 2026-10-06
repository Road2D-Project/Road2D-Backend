package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubMembership struct {
	trip    *model.Trip
	member  *model.TripMember
	pending []model.TripMember
	added   *model.TripMember
	updated *model.TripMember
	kicked  *model.TripMember
	active  int
	findErr error
}

func (s *stubMembership) AssignBranch(context.Context, *model.TripMember, uuid.UUID) error {
	return nil
}

func (s *stubMembership) FindTripByID(context.Context, uuid.UUID) (*model.Trip, error) {
	if s.trip == nil {
		return nil, repository.ErrTripNotFound
	}
	copied := *s.trip
	return &copied, nil
}

func (s *stubMembership) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.TripMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotTripMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubMembership) UpdateMember(_ context.Context, member *model.TripMember) error {
	copied := *member
	s.updated = &copied
	return nil
}

func (s *stubMembership) AddMember(_ context.Context, userId, tripId uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, invitorName *string) (*model.TripMember, error) {
	s.added = &model.TripMember{
		UserID: userId, TripID: tripId, Role: role, Status: status, Nickname: nickname, InvitorName: invitorName,
	}
	return s.added, nil
}

func (s *stubMembership) CountActiveMembers(context.Context, uuid.UUID) (int, error) {
	return s.active, nil
}

func (s *stubMembership) ListPendingMembers(context.Context, uuid.UUID) ([]model.TripMember, error) {
	return s.pending, nil
}

func (s *stubMembership) ListActiveMembers(context.Context, uuid.UUID) ([]model.TripMember, error) {
	if s.member == nil {
		return nil, nil
	}
	return []model.TripMember{*s.member}, nil
}

func (s *stubMembership) ApplyKick(_ context.Context, member *model.TripMember) error {
	copied := *member
	s.kicked = &copied
	return nil
}

type stubRoleCache struct {
	setRole enum.TripRole
	deleted bool
}

func (s *stubRoleCache) SetCachedTripMemberRole(_ context.Context, _, _ uuid.UUID, role enum.TripRole) error {
	s.setRole = role
	return nil
}

func (s *stubRoleCache) DeleteCachedTripMemberRole(context.Context, uuid.UUID, uuid.UUID) error {
	s.deleted = true
	return nil
}

func TestMemberInviteStaysPendingUntilApproval(t *testing.T) {
	store := &stubMembership{trip: &model.Trip{MemberLimit: 15}}
	invitor, invited := uuid.New(), uuid.New()
	got, err := NewInviteTripMemberService(store, store).Invite(context.Background(), invited, uuid.New(), invitor, "ann", enum.TripRoleMember)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipPending || store.added.InvitorName == nil || *store.added.InvitorName != "ann" {
		t.Fatalf("got %+v added %+v", got, store.added)
	}
}

func TestLeaderInviteWaitsForTheInvitee(t *testing.T) {
	store := &stubMembership{trip: &model.Trip{MemberLimit: 15}}
	got, err := NewInviteTripMemberService(store, store).Invite(context.Background(), uuid.New(), uuid.New(), uuid.New(), "lead", enum.TripRoleLeader)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipInvited {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestAcceptJoinRequestSeatsTheUser(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	store := &stubMembership{
		trip:   &model.Trip{MemberLimit: 15},
		member: &model.TripMember{UserID: userID, TripID: tripID, Role: enum.TripRoleMember, Status: enum.MembershipPending},
		active: 2,
	}
	cache := &stubRoleCache{}
	got, err := NewTripJoinRequestService(store, store, cache).Accept(context.Background(), userID, tripID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipActive || cache.setRole != enum.TripRoleMember {
		t.Fatalf("got %+v cache %s", got, cache.setRole)
	}
}

func TestAcceptJoinRequestStopsAtTheCap(t *testing.T) {
	store := &stubMembership{
		trip:   &model.Trip{MemberLimit: 15},
		member: &model.TripMember{Status: enum.MembershipPending, Role: enum.TripRoleMember},
		active: 15,
	}
	_, err := NewTripJoinRequestService(store, store, nil).Accept(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrTripMemberLimit) {
		t.Fatalf("got %v", err)
	}
}

func TestKickRefusesTheLeader(t *testing.T) {
	store := &stubMembership{member: &model.TripMember{Status: enum.MembershipActive, Role: enum.TripRoleLeader}}
	err := NewKickTripMemberService(store, &stubRoleCache{}).Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrCannotKickLeader) {
		t.Fatalf("got %v", err)
	}
}

func TestKickAdminCannotRemoveAnotherAdmin(t *testing.T) {
	store := &stubMembership{member: &model.TripMember{Status: enum.MembershipActive, Role: enum.TripRoleAdmin}}
	err := NewKickTripMemberService(store, nil).Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.TripRoleAdmin)
	if !errors.Is(err, repository.ErrInsufficientKickRole) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateNicknameIsOnlyForYourself(t *testing.T) {
	nickname := "scout"
	store := &stubMembership{member: &model.TripMember{Status: enum.MembershipActive, Role: enum.TripRoleMember}}
	_, err := NewTripMemberService(store, nil).Update(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.TripRoleLeader, request.UpdateTripMemberRequest{
		Nickname: &nickname,
	})
	if !errors.Is(err, repository.ErrCannotUpdateOtherNickname) {
		t.Fatalf("got %v", err)
	}
}

func TestLeaderCanAppointAdmin(t *testing.T) {
	actor, target, tripID := uuid.New(), uuid.New(), uuid.New()
	role := enum.TripRoleAdmin
	store := &stubMembership{member: &model.TripMember{UserID: target, Status: enum.MembershipActive, Role: enum.TripRoleMember}}
	cache := &stubRoleCache{}
	got, err := NewTripMemberService(store, cache).Update(context.Background(), actor, target, tripID, enum.TripRoleLeader, request.UpdateTripMemberRequest{
		Role: &role,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != enum.TripRoleAdmin || cache.setRole != enum.TripRoleAdmin {
		t.Fatalf("got %+v cache %s", got, cache.setRole)
	}
}
