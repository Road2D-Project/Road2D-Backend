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

type stubJoinMembers struct {
	member    *model.TripMember
	findErr   error
	added     *model.TripMember
	updated   *model.TripMember
	addErr    error
	updateErr error
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

func (s *stubJoinMembers) AddActiveMember(_ context.Context, userId, tripId uuid.UUID, role enum.TripRole, nickname string) (*model.TripMember, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.added = &model.TripMember{UserID: userId, TripID: tripId, Role: role, Nickname: nickname, Status: enum.MembershipActive}
	return s.added, nil
}

func TestJoinTripInsertsNewMember(t *testing.T) {
	token := uuid.New().String()
	tripID := uuid.New()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token}}}
	trips.byToken[token].ID = tripID
	members := &stubJoinMembers{}
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()

	got, err := NewJoinTripService(trips, members, nil).Join(context.Background(), user, token)
	if err != nil {
		t.Fatal(err)
	}
	if got.MyRole == nil || *got.MyRole != enum.TripRoleMember {
		t.Logf("Hello?")
		t.Fatal("joiner should be member")
	}
	if members.added == nil || members.added.UserID != user.ID || members.added.TripID != tripID {
		t.Fatalf("added = %+v", members.added)
	}

}

func TestJoinTripReactivatesLeftMember(t *testing.T) {
	token := uuid.New().String()
	tripID, userID := uuid.New(), uuid.New()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token}}}
	trips.byToken[token].ID = tripID
	members := &stubJoinMembers{member: &model.TripMember{
		UserID: userID, TripID: tripID, Role: enum.TripRoleMember, Status: enum.MembershipLeft,
	}}
	user := &authModel.User{Username: "scout"}
	user.ID = userID

	if _, err := NewJoinTripService(trips, members, nil).Join(context.Background(), user, token); err != nil {
		t.Fatal(err)
	}
	if members.updated == nil || members.updated.Status != enum.MembershipActive {
		t.Fatalf("updated = %+v", members.updated)
	}
	if members.added != nil {
		t.Fatal("left row should be reused, not inserted")
	}
}

func TestJoinTripRejectsActiveMember(t *testing.T) {
	token := uuid.New().String()
	trips := &stubTripRecords{byToken: map[string]*model.Trip{token: {InviteToken: token}}}
	members := &stubJoinMembers{member: &model.TripMember{Status: enum.MembershipActive}}
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	err := error(nil)
	_, err = NewJoinTripService(trips, members, nil).Join(context.Background(), user, token)
	if !errors.Is(err, repository.ErrAlreadyTripMember) {
		t.Fatalf("got %v", err)
	}
}

func TestJoinTripUnknownToken(t *testing.T) {
	user := &authModel.User{Username: "scout"}
	user.ID = uuid.New()
	_, err := NewJoinTripService(&stubTripRecords{byToken: map[string]*model.Trip{}}, &stubJoinMembers{}, nil).
		Join(context.Background(), user, uuid.New().String())
	if !errors.Is(err, repository.ErrInviteNotFound) {
		t.Fatalf("got %v", err)
	}
}
