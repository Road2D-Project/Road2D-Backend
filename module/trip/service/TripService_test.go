package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	groupModel "Road-To-Destination-BE/module/group/model"
	groupRepo "Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubTripRecords struct {
	created     *model.Trip
	members     []model.TripMember
	byID        map[uuid.UUID]*model.Trip
	byToken     map[string]*model.Trip
	listed      []repository.ActiveTripByUser
	inviteToken string
	deleted     uuid.UUID
}

func (s *stubTripRecords) CreateTripWithMembers(_ context.Context, trip *model.Trip, members []model.TripMember) error {
	if trip.ID == uuid.Nil {
		trip.ID = uuid.New()
	}
	copied := *trip
	s.created = &copied
	s.members = append([]model.TripMember(nil), members...)
	if s.byID == nil {
		s.byID = map[uuid.UUID]*model.Trip{}
	}
	if s.byToken == nil {
		s.byToken = map[string]*model.Trip{}
	}
	s.byID[trip.ID] = &copied
	s.byToken[trip.InviteToken] = &copied
	return nil
}

func (s *stubTripRecords) FindTripByID(_ context.Context, id uuid.UUID) (*model.Trip, error) {
	trip, ok := s.byID[id]
	if !ok {
		return nil, repository.ErrTripNotFound
	}
	copied := *trip
	return &copied, nil
}

func (s *stubTripRecords) FindTripByInviteToken(_ context.Context, token string) (*model.Trip, error) {
	trip, ok := s.byToken[token]
	if !ok {
		return nil, repository.ErrInviteNotFound
	}
	copied := *trip
	return &copied, nil
}

func (s *stubTripRecords) UpdateTripInfo(_ context.Context, id uuid.UUID, updates map[string]any) error {
	trip, ok := s.byID[id]
	if !ok {
		return repository.ErrTripNotFound
	}
	if name, ok := updates["name"].(string); ok {
		trip.Name = name
	}
	if note, ok := updates["note"].(string); ok {
		trip.Note = note
	}
	return nil
}

func (s *stubTripRecords) UpdateInviteToken(_ context.Context, id uuid.UUID, token string) error {
	trip, ok := s.byID[id]
	if !ok {
		return repository.ErrTripNotFound
	}
	if s.byToken != nil {
		delete(s.byToken, trip.InviteToken)
		s.byToken[token] = trip
	}
	trip.InviteToken = token
	s.inviteToken = token
	return nil
}

func (s *stubTripRecords) DeleteTrip(_ context.Context, id uuid.UUID) error {
	s.deleted = id
	return nil
}

func (s *stubTripRecords) ListActiveTripByUser(context.Context, uuid.UUID) ([]repository.ActiveTripByUser, error) {
	return s.listed, nil
}

func (s *stubTripRecords) FindDestinationsByIDs(context.Context, []uuid.UUID) ([]model.Destination, error) {
	return nil, nil
}

func (s *stubTripRecords) ReplaceTripBranches(context.Context, uuid.UUID, []model.TripBranch) error {
	return nil
}

func (s *stubTripRecords) FindTripWithBranches(context.Context, uuid.UUID) (*model.Trip, error) {
	return nil, repository.ErrTripNotFound
}

type stubGroups struct {
	byID map[uuid.UUID]*groupModel.Group
}

func (s stubGroups) FindGroupByID(_ context.Context, id uuid.UUID) (*groupModel.Group, error) {
	g, ok := s.byID[id]
	if !ok {
		return nil, groupRepo.ErrGroupNotFound
	}
	return g, nil
}

type stubGroupRoles struct {
	err error
}

func (s stubGroupRoles) FindGroupActiveMemberRole(context.Context, uuid.UUID, uuid.UUID) (enum.GroupRole, error) {
	if s.err != nil {
		return 0, s.err
	}
	return enum.GroupRoleMember, nil
}

func TestCreateTripCallerBecomesLeader(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	groupID := uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubGroups{byID: map[uuid.UUID]*groupModel.Group{groupID: {}}}, stubGroupRoles{}, nil, nil)

	got, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		GroupID: groupID,
		Name:    " Ha Giang ",
		Note:    " helmets ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ha Giang" || trips.created.Note != "helmets" {
		t.Fatalf("got %+v", got)
	}
	if got.MyRole == nil || *got.MyRole != enum.TripRoleLeader {
		t.Fatal("creator should be leader")
	}
	if got.OwnerID != owner.ID || got.GroupID == nil || *got.GroupID != groupID {
		t.Fatal("owner and group should be set")
	}
	if trips.created.InviteToken == "" {
		t.Fatal("invite token should be minted on create")
	}
	if len(trips.members) != 1 || trips.members[0].Role != enum.TripRoleLeader {
		t.Fatalf("members = %+v", trips.members)
	}
}

func TestCreateTripUnknownGroupAborts(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, stubGroups{byID: map[uuid.UUID]*groupModel.Group{}}, stubGroupRoles{}, nil, nil)
	_, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		GroupID: uuid.New(),
		Name:    "Loop",
	})
	if !errors.Is(err, repository.ErrGroupNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripRequiresGroupMembership(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	groupID := uuid.New()
	svc := NewTripService(
		&stubTripRecords{},
		stubGroups{byID: map[uuid.UUID]*groupModel.Group{groupID: {}}},
		stubGroupRoles{err: groupRepo.ErrUserNotGroupMember},
		nil,
		nil,
	)
	_, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		GroupID: groupID,
		Name:    "Loop",
	})
	if !errors.Is(err, repository.ErrUserNotGroupMember) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateTripRequiresAField(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{id: {Name: "a"}}}, nil, nil, nil, nil)
	_, err := svc.UpdateTrip(context.Background(), id, request.UpdateTripRequest{}, enum.TripRoleLeader)
	if !errors.Is(err, repository.ErrNoTripUpdate) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateInviteLinkRotatesToken(t *testing.T) {
	id := uuid.New()
	old := uuid.New().String()
	trips := &stubTripRecords{
		byID:    map[uuid.UUID]*model.Trip{id: {InviteToken: old}},
		byToken: map[string]*model.Trip{old: {InviteToken: old}},
	}
	trips.byID[id].ID = id
	svc := NewTripService(trips, nil, nil, nil, nil)
	got, err := svc.CreateInviteLink(context.Background(), id, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.Token == old || got.Token == "" {
		t.Fatalf("token = %q", got.Token)
	}
	if got.JoinPath != "/v1/trips/join/"+got.Token {
		t.Fatalf("joinPath = %q", got.JoinPath)
	}
}
