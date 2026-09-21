package service

import (
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubLeaveTripMembers struct {
	member    *model.TripMember
	roster    []model.TripMember
	findErr   error
	leaveErr  error
	leaver    *model.TripMember
	successor *model.TripMember
}

func (s *stubLeaveTripMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.TripMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotTripMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubLeaveTripMembers) ListActiveMembers(context.Context, uuid.UUID) ([]model.TripMember, error) {
	return s.roster, nil
}

func (s *stubLeaveTripMembers) ApplyLeave(_ context.Context, leaver *model.TripMember, successor *model.TripMember) error {
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

type stubLeaveTripCache struct {
	deletedUser uuid.UUID
	setUser     uuid.UUID
	setRole     enum.TripRole
	deleted     bool
}

func (s *stubLeaveTripCache) SetCachedTripMemberRole(_ context.Context, _, userId uuid.UUID, role enum.TripRole) error {
	s.setUser = userId
	s.setRole = role
	return nil
}

func (s *stubLeaveTripCache) DeleteCachedTripMemberRole(_ context.Context, _, userId uuid.UUID) error {
	s.deleted = true
	s.deletedUser = userId
	return nil
}

func atHour(hours int) *time.Time {
	t := time.Date(2026, 9, 21, hours, 0, 0, 0, time.UTC)
	return &t
}

func TestLeaveMarksActiveMemberLeft(t *testing.T) {
	userID, tripID := uuid.New(), uuid.New()
	members := &stubLeaveTripMembers{member: &model.TripMember{
		UserID: userID, TripID: tripID, Role: enum.TripRoleMember, Status: enum.MembershipActive,
	}}
	cache := &stubLeaveTripCache{}
	svc := NewLeaveTripService(members, cache)

	if err := svc.Leave(context.Background(), userID, tripID); err != nil {
		t.Fatal(err)
	}
	if members.successor != nil {
		t.Fatal("member leave must not transfer leadership")
	}
	if !cache.deleted || cache.deletedUser != userID {
		t.Fatal("leave should drop the caller's cached role")
	}
}

func TestLeaveLeaderTransfersToEarliestMember(t *testing.T) {
	leaderID, lateID, earlyID := uuid.New(), uuid.New(), uuid.New()
	tripID := uuid.New()
	members := &stubLeaveTripMembers{
		member: &model.TripMember{UserID: leaderID, TripID: tripID, Role: enum.TripRoleLeader, Status: enum.MembershipActive},
		roster: []model.TripMember{
			{UserID: lateID, Role: enum.TripRoleMember, Status: enum.MembershipActive, JoinedAt: atHour(9)},
			{UserID: leaderID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: atHour(1)},
			{UserID: earlyID, Role: enum.TripRoleMember, Status: enum.MembershipActive, JoinedAt: atHour(5)},
		},
	}
	cache := &stubLeaveTripCache{}
	svc := NewLeaveTripService(members, cache)

	if err := svc.Leave(context.Background(), leaderID, tripID); err != nil {
		t.Fatal(err)
	}
	if members.successor == nil || members.successor.UserID != earlyID {
		t.Fatalf("successor = %+v, want earliest member", members.successor)
	}
	if cache.setUser != earlyID || cache.setRole != enum.TripRoleLeader {
		t.Fatal("new leader role should be cached")
	}
}

func TestLeaveLeaderAloneHasNoSuccessor(t *testing.T) {
	leaderID, tripID := uuid.New(), uuid.New()
	svc := NewLeaveTripService(&stubLeaveTripMembers{
		member: &model.TripMember{UserID: leaderID, TripID: tripID, Role: enum.TripRoleLeader, Status: enum.MembershipActive},
		roster: []model.TripMember{
			{UserID: leaderID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: atHour(1)},
		},
	}, nil)
	err := svc.Leave(context.Background(), leaderID, tripID)
	if !errors.Is(err, repository.ErrNoSuccessorToTransfer) {
		t.Fatalf("got %v", err)
	}
}

func TestLeaveInactiveMemberIsForbidden(t *testing.T) {
	svc := NewLeaveTripService(&stubLeaveTripMembers{member: &model.TripMember{
		Status: enum.MembershipLeft, Role: enum.TripRoleMember,
	}}, nil)
	err := svc.Leave(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, repository.ErrUserNotTripMember) {
		t.Fatalf("got %v", err)
	}
}
