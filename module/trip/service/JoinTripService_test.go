package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// trỏ đúng path -> go test -run TestXxxxx -v -count=1
type stubJoinMembers struct {
	member    *model.TripMember
	findErr   error
	added     *model.TripMember
	updated   *model.TripMember
	addErr    error
	updateErr error
	active    int
}

func (s *stubJoinMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.TripMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotTripMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubJoinMembers) UpdateMember(_ context.Context, member *model.TripMember) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	copied := *member
	s.updated = &copied
	return nil
}

func (s *stubJoinMembers) AddMember(_ context.Context, userId, tripId uuid.UUID, role enum.TripRole, status enum.MembershipStatus, nickname string, _ *string) (*model.TripMember, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.added = &model.TripMember{UserID: userId, TripID: tripId, Role: role, Nickname: nickname, Status: status}
	return s.added, nil
}

func (s *stubJoinMembers) CountActiveMembers(context.Context, uuid.UUID) (int, error) {
	return s.active, nil
}

func TestJoinTripInsertsNewMember(t *testing.T) {
	token := uuid.New().String()
	tripID := uuid.New()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token, MemberLimit: 15}}}
	trips.byToken[token].ID = tripID
	members := &stubJoinMembers{}
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	got, err := NewJoinTripService(trips, members).Join(context.Background(), user, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.MembershipPending || got.Role != enum.TripRoleMember {
		t.Fatalf("got %+v", got)
	}
	if members.added == nil || members.added.UserID != user.ID || members.added.TripID != tripID || members.added.Status != enum.MembershipPending {
		t.Fatalf("added = %+v", members.added)
	}
}

func TestJoinTripReactivatesLeftMember(t *testing.T) {
	token := uuid.New().String()
	tripID, userID := uuid.New(), uuid.New()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token, MemberLimit: 15}}}
	trips.byToken[token].ID = tripID
	members := &stubJoinMembers{member: &model.TripMember{
		UserID: userID, TripID: tripID, Role: enum.TripRoleMember, Status: enum.MembershipLeft,
	}}
	user := &authModel.User{Username: "scout"}
	user.ID = userID

	if _, err := NewJoinTripService(trips, members).Join(context.Background(), user, token); err != nil {
		t.Fatal(err)
	}
	if members.updated == nil || members.updated.Status != enum.MembershipPending {
		t.Fatalf("updated = %+v", members.updated)
	}
	if members.added != nil {
		t.Fatal("left row should be reused, not inserted")
	}
}

func TestJoinTripRejectsActiveMember(t *testing.T) {
	token := uuid.New().String()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token, MemberLimit: 15}}}
	members := &stubJoinMembers{member: &model.TripMember{Status: enum.MembershipActive}}
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	_, err := NewJoinTripService(trips, members).Join(context.Background(), user, token)
	if !errors.Is(err, repository.ErrAlreadyTripMember) {
		t.Fatalf("got %v", err)
	}
}

func TestJoinTripUnknownToken(t *testing.T) {
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	_, err := NewJoinTripService(&stubTripRecords{byToken: map[string]*model.Trip{}}, &stubJoinMembers{}).
		Join(context.Background(), user, uuid.New().String())
	if !errors.Is(err, repository.ErrInviteNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestJoinTripRefusesWhenFull(t *testing.T) {
	token := uuid.New().String()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token, MemberLimit: 15}}}
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	_, err := NewJoinTripService(trips, &stubJoinMembers{active: 15}).Join(context.Background(), user, token)
	if !errors.Is(err, repository.ErrTripMemberLimit) {
		t.Fatalf("got %v", err)
	}
}
