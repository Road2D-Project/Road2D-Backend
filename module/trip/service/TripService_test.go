package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubTripRecords struct {
	created     *model.Trip
	members     []model.TripMember
	main        *model.InitialMainBranch
	byID        map[uuid.UUID]*model.Trip
	byToken     map[string]*model.Trip
	byOwnerKey  map[string]*model.Trip
	createCalls int
	loseKeyOnce bool
	clashOnce   bool
	listed      []repository.ActiveTripByUser
	publicTrips []model.Trip
	inviteToken string
	deleted     uuid.UUID
}

func (s *stubTripRecords) CreateTripWithMembers(_ context.Context, trip *model.Trip, members []model.TripMember, main *model.InitialMainBranch) error {
	if trip.ID == uuid.Nil {
		trip.ID = uuid.New()
	}
	s.createCalls++
	if s.clashOnce {
		s.clashOnce = false
		clash := *trip
		clash.IdempotencyHash = "other-body"
		s.storeTrip(&clash)
		return repository.ErrIdempotencyKeyTaken
	}
	if s.loseKeyOnce {
		s.loseKeyOnce = false
		s.storeTrip(trip)
		return repository.ErrIdempotencyKeyTaken
	}
	s.storeTrip(trip)
	s.members = append([]model.TripMember(nil), members...)
	s.main = main
	return nil
}

func (s *stubTripRecords) storeTrip(trip *model.Trip) {
	copied := *trip
	if s.byID == nil {
		s.byID = map[uuid.UUID]*model.Trip{}
	}
	if s.byToken == nil {
		s.byToken = map[string]*model.Trip{}
	}
	if s.byOwnerKey == nil {
		s.byOwnerKey = map[string]*model.Trip{}
	}
	s.byID[copied.ID] = &copied
	if copied.InviteToken != "" {
		s.byToken[copied.InviteToken] = &copied
	}
	if copied.OwnerID != uuid.Nil && copied.IdempotencyKey != "" {
		s.byOwnerKey[copied.OwnerID.String()+"|"+copied.IdempotencyKey] = &copied
	}
	s.created = &copied
}

func (s *stubTripRecords) FindTripByOwnerIdempotencyKey(_ context.Context, ownerID uuid.UUID, key string) (*model.Trip, error) {
	if s.byOwnerKey == nil {
		return nil, repository.ErrTripNotFound
	}
	trip, ok := s.byOwnerKey[ownerID.String()+"|"+key]
	if !ok {
		return nil, repository.ErrTripNotFound
	}
	copied := *trip
	return &copied, nil
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
	if visibility, ok := updates["visibility"].(bool); ok {
		trip.Visibility = visibility
	}
	if status, ok := updates["status"].(enum.TripStatus); ok {
		trip.Status = status
	}
	return nil
}

func (s *stubTripRecords) CompleteTrip(_ context.Context, id uuid.UUID) error {
	trip, ok := s.byID[id]
	if !ok {
		return repository.ErrTripNotFound
	}
	trip.Status = enum.TripCompleted
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

func (s *stubTripRecords) ListPublicTrips(context.Context, int) ([]model.Trip, error) {
	return s.publicTrips, nil
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

func (s *stubTripRecords) UpdateBranchStops(context.Context, model.TripBranch) error {
	return nil
}

func (s *stubTripRecords) UpdateBranchInfo(context.Context, uuid.UUID, uuid.UUID, map[string]any) error {
	return nil
}

func (s *stubTripRecords) ListTripIDsByDestination(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return []uuid.UUID{}, nil
}

func (s *stubTripRecords) DeleteDestination(context.Context, uuid.UUID) error {
	return nil
}

type stubUsersByID struct {
	byID map[uuid.UUID]*authModel.User
}

func (s stubUsersByID) FindUserByID(_ context.Context, id uuid.UUID) (*authModel.User, error) {
	user, ok := s.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

type stubRoster struct {
	members []model.TripMember
}

func (s stubRoster) ListActiveMembers(context.Context, uuid.UUID) ([]model.TripMember, error) {
	return s.members, nil
}

type stubActiveTripRole struct {
	err error
}

func (s stubActiveTripRole) FindTripActiveMemberRole(context.Context, uuid.UUID, uuid.UUID) (enum.TripRole, error) {
	if s.err != nil {
		return 0, s.err
	}
	return enum.TripRoleMember, nil
}

func bronzeType() *enum.TripType {
	tripType := enum.TripTypeBronze
	return &tripType
}

func TestCreateTripCallerBecomesLeaderAndSeatsMembers(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})

	got, created, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           " Ha Giang ",
		Note:           " helmets ",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID, owner.ID, member.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("first create should insert")
	}
	if got.Name != "Ha Giang" || trips.created.Note != "helmets" {
		t.Fatalf("got %+v", got)
	}
	if got.MyRole == nil || *got.MyRole != enum.TripRoleLeader {
		t.Fatal("creator should be leader")
	}
	if got.OwnerID != owner.ID || got.TripType != enum.TripTypeBronze || got.MemberLimit != 15 || got.Visibility {
		t.Fatalf("trip fields = %+v", got)
	}
	if trips.created.InviteToken == "" || trips.created.IdempotencyKey == "" || trips.created.IdempotencyHash == "" {
		t.Fatal("invite token and idempotency key should be stored on create")
	}
	if len(trips.members) != 2 || trips.members[0].Role != enum.TripRoleLeader || trips.members[1].Role != enum.TripRoleMember || trips.members[1].UserID != member.ID {
		t.Fatalf("members = %+v", trips.members)
	}
}

func TestCreateTripReplaysTheSameIdempotencyKey(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})
	key := uuid.New().String()
	firstBody := request.CreateTripRequest{
		IdempotencyKey: key,
		Name:           " Ha Giang ",
		Note:           " helmets ",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID, owner.ID, member.ID},
	}
	first, inserted, err := svc.CreateTrip(context.Background(), owner, firstBody)
	if err != nil {
		t.Fatal(err)
	}
	secondBody := firstBody
	secondBody.Name = "Ha Giang"
	secondBody.Note = "helmets"
	secondBody.MemberUserIDs = []uuid.UUID{member.ID}
	second, replayedInsert, err := svc.CreateTrip(context.Background(), owner, secondBody)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || replayedInsert {
		t.Fatal("only the first call should insert")
	}
	if second.ID != first.ID || trips.createCalls != 1 {
		t.Fatalf("replay id %s calls %d", second.ID, trips.createCalls)
	}
}

func TestCreateTripRejectsSameKeyWithADifferentBody(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})
	key := uuid.New().String()
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: key,
		Name:           "Ha Giang",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: key,
		Name:           "Other road",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	})
	if !errors.Is(err, repository.ErrIdempotencyConflict) {
		t.Fatalf("got %v", err)
	}
	if trips.createCalls != 1 {
		t.Fatalf("create calls = %d", trips.createCalls)
	}
}

func TestCreateTripReplaysWhenInsertLosesTheKey(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{loseKeyOnce: true}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})
	got, inserted, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Ha Giang",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if inserted || got.ID != trips.created.ID {
		t.Fatalf("inserted %v id %s stored %s", inserted, got.ID, trips.created.ID)
	}
}

func TestCreateTripConflictWhenInsertLosesToADifferentBody(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{clashOnce: true}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Ha Giang",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	})
	if !errors.Is(err, repository.ErrIdempotencyConflict) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripSameKeyIsPerOwner(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	other := &authModel.User{Username: "other"}
	other.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{})
	body := request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Ha Giang",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	}
	first, _, err := svc.CreateTrip(context.Background(), owner, body)
	if err != nil {
		t.Fatal(err)
	}
	second, inserted, err := svc.CreateTrip(context.Background(), other, body)
	if err != nil {
		t.Fatal(err)
	}
	if !inserted || second.ID == first.ID || trips.createCalls != 2 {
		t.Fatalf("second %+v calls %d", second, trips.createCalls)
	}
}

func TestCreateTripRequiresIdempotencyKey(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{}, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		Name:          "Loop",
		TripType:      bronzeType(),
		MemberUserIDs: []uuid.UUID{uuid.New()},
	})
	if !errors.Is(err, repository.ErrIdempotencyKeyRequired) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripRequiresAnotherMember(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{}, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{owner.ID},
	})
	if !errors.Is(err, repository.ErrTripMembersRequired) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripUnknownMemberAborts(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{}, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{uuid.New()},
	})
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripRejectsTierWithoutPolicy(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	memberID := uuid.New()
	silver := enum.TripTypeSilver
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{byID: map[uuid.UUID]*authModel.User{
		memberID: {Username: "scout"},
	}}, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       &silver,
		MemberUserIDs:  []uuid.UUID{memberID},
	})
	if !errors.Is(err, repository.ErrTripTypePolicyUnset) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripRejectsRosterPastBronzeLimit(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	users := stubUsersByID{byID: map[uuid.UUID]*authModel.User{}}
	ids := make([]uuid.UUID, 0, 15)
	for i := 0; i < 15; i++ {
		id := uuid.New()
		ids = append(ids, id)
		account := &authModel.User{Username: "m"}
		account.ID = id
		users.byID[id] = account
	}
	svc := NewTripService(&stubTripRecords{}, users, nil, nil, nil, nil, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       bronzeType(),
		MemberUserIDs:  ids,
	})
	if !errors.Is(err, repository.ErrTripMemberLimit) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateTripRequiresAField(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{id: {Name: "a"}}}, nil, nil, nil, nil, nil, stubAcceptedFriends{})
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
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
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

func TestUpdateTripChangesVisibilityOnly(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{id: {Name: "a"}}}
	trips.byID[id].ID = id
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	public := true
	got, err := svc.UpdateTrip(context.Background(), id, request.UpdateTripRequest{Visibility: &public}, enum.TripRoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Visibility || got.MemberLimit != 0 {
		t.Fatalf("visibility = %v limit = %d", got.Visibility, got.MemberLimit)
	}
}

func TestGetTripHidesPrivateFromOutsiders(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{id: {Name: "closed", Visibility: false}}}
	trips.byID[id].ID = id
	svc := NewTripService(trips, nil, nil, stubActiveTripRole{err: repository.ErrUserNotTripMember}, nil, nil, stubAcceptedFriends{})
	_, err := svc.GetTrip(context.Background(), id, uuid.New())
	if !errors.Is(err, repository.ErrTripNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestGetTripShowsPublicTripToOutsiders(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{id: {Name: "open", Visibility: true}}}
	trips.byID[id].ID = id
	svc := NewTripService(trips, nil, nil, stubActiveTripRole{err: repository.ErrUserNotTripMember}, nil, nil, stubAcceptedFriends{})
	got, err := svc.GetTrip(context.Background(), id, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if got.MyRole != nil || got.Name != "open" {
		t.Fatalf("got %+v", got)
	}
}

func TestListPublicTripsOmitsPrivate(t *testing.T) {
	public := model.Trip{Name: "open", Visibility: true}
	trips := &stubTripRecords{publicTrips: []model.Trip{public}}
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	got, err := svc.ListPublicTrips(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Trips) != 1 || !got.Trips[0].Visibility {
		t.Fatalf("got %+v", got)
	}
}

func TestForkTripDropsExcludedMembersAndMakesCallerLeader(t *testing.T) {
	caller := &authModel.User{Username: "forker"}
	caller.ID = uuid.New()
	keep := uuid.New()
	drop := uuid.New()
	sourceID := uuid.New()
	source := &model.Trip{
		Name: "source", Note: "keep", TripType: enum.TripTypeBronze, MemberLimit: 15, Visibility: true,
	}
	source.ID = sourceID
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{sourceID: source}}
	roster := stubRoster{members: []model.TripMember{
		{UserID: caller.ID, Role: enum.TripRoleMember, Status: enum.MembershipActive, Nickname: "forker"},
		{UserID: keep, Role: enum.TripRoleAdmin, Status: enum.MembershipActive, Nickname: "keep"},
		{UserID: drop, Role: enum.TripRoleMember, Status: enum.MembershipActive, Nickname: "drop"},
	}}
	svc := NewTripService(trips, nil, roster, stubActiveTripRole{}, nil, nil, stubAcceptedFriends{})
	got, err := svc.ForkTrip(context.Background(), caller, sourceID, request.ForkTripRequest{
		ExcludeUserIDs: []uuid.UUID{drop, caller.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.MyRole == nil || *got.MyRole != enum.TripRoleLeader || got.Name != "source" || !got.Visibility {
		t.Fatalf("got %+v", got)
	}
	if len(trips.members) != 2 || trips.members[0].UserID != caller.ID || trips.members[0].Role != enum.TripRoleLeader {
		t.Fatalf("leader = %+v", trips.members)
	}
	if trips.members[1].UserID != keep || trips.members[1].Role != enum.TripRoleMember {
		t.Fatalf("copied = %+v", trips.members[1])
	}
}

func reviewedPair(from, to model.Location) *response.PreviewLocationsResponse {
	return &response.PreviewLocationsResponse{
		Locations: []response.PreviewLocation{
			{LocationID: from.ID, Name: "client", Lat: 0, Lng: 0},
			{LocationID: to.ID, Name: "client", Lat: 0, Lng: 0},
		},
		Legs: []response.ComputeBranchLeg{{
			From:      response.ComputeBranchStop{Name: from.Name, LocationID: &from.ID},
			To:        response.ComputeBranchStop{Name: to.Name, LocationID: &to.ID},
			Vehicle:   enum.BIKE,
			Polyline:  "reviewed",
			DistanceM: 12,
			DurationS: 3,
		}},
	}
}

func TestCreateTripStoresReviewedMainBranch(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	from := model.Location{Name: "cafe", Lat: 10.5, Lng: 20.25}
	from.ID = uuid.New()
	to := model.Location{Name: "park", Lat: 11, Lng: 21}
	to.ID = uuid.New()
	trips := &stubTripRecords{}
	svc := NewTripService(trips, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, &stubLocations{
		byID: map[uuid.UUID]*model.Location{from.ID: &from, to.ID: &to},
	}, stubAcceptedFriends{})

	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Ha Giang",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
		MainBranch:     reviewedPair(from, to),
	})
	if err != nil {
		t.Fatal(err)
	}
	if trips.main == nil || len(trips.main.Destinations) != 2 || len(trips.main.Travels) != 1 {
		t.Fatalf("main = %+v", trips.main)
	}
	if trips.main.Destinations[0].Name != "cafe" || trips.main.Destinations[0].Lat != 10.5 || trips.main.Destinations[0].LocationID == nil || *trips.main.Destinations[0].LocationID != from.ID {
		t.Fatalf("first pin = %+v", trips.main.Destinations[0])
	}
	if trips.main.Destinations[1].Name != "park" || trips.main.Travels[0].Polyline != "reviewed" || trips.main.Travels[0].FromDestinationID != trips.main.Destinations[0].ID {
		t.Fatalf("hop = %+v", trips.main.Travels[0])
	}
}

func TestCreateTripRejectsMisalignedMainBranch(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	from := model.Location{Name: "cafe", Lat: 1, Lng: 2}
	from.ID = uuid.New()
	to := model.Location{Name: "park", Lat: 3, Lng: 4}
	to.ID = uuid.New()
	other := uuid.New()
	branch := reviewedPair(from, to)
	branch.Legs[0].To.LocationID = &other
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, &stubLocations{
		byID: map[uuid.UUID]*model.Location{from.ID: &from, to.ID: &to},
	}, stubAcceptedFriends{})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
		MainBranch:     branch,
	})
	if !errors.Is(err, repository.ErrInvalidTripGraph) {
		t.Fatalf("got %v", err)
	}
}

func TestCreateTripRejectsMemberWhoIsNotAFriend(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	member := &authModel.User{Username: "scout"}
	member.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, stubUsersByID{byID: map[uuid.UUID]*authModel.User{member.ID: member}}, nil, nil, nil, nil, stubAcceptedFriends{
		missing: []uuid.UUID{member.ID},
	})
	_, _, err := svc.CreateTrip(context.Background(), owner, request.CreateTripRequest{
		IdempotencyKey: uuid.New().String(),
		Name:           "Loop",
		TripType:       bronzeType(),
		MemberUserIDs:  []uuid.UUID{member.ID},
	})
	if !errors.Is(err, repository.ErrMemberNotFriend) {
		t.Fatalf("got %v", err)
	}
}

type stubAcceptedFriends struct {
	missing []uuid.UUID
}

func (s stubAcceptedFriends) MissingFriends(context.Context, uuid.UUID, []uuid.UUID) ([]uuid.UUID, error) {
	return s.missing, nil
}

func TestStartTripActivatesAPlanningTrip(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripPlanning},
	}}
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	if err := svc.StartTrip(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if trips.byID[id].Status != enum.TripActive {
		t.Fatalf("status %s", trips.byID[id].Status)
	}
}

func TestStartTripRejectsACompletedTrip(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripCompleted},
	}}, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	err := svc.StartTrip(context.Background(), id)
	if !errors.Is(err, repository.ErrTripAlreadyCompleted) {
		t.Fatalf("got %v", err)
	}
}

func TestStartTripRejectsATripThatIsAlreadyUnderway(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripActive},
	}}, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	err := svc.StartTrip(context.Background(), id)
	if !errors.Is(err, repository.ErrTripCannotActivate) {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteTripLocksAnActiveTrip(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripActive},
	}}
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	if err := svc.CompleteTrip(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if trips.byID[id].Status != enum.TripCompleted {
		t.Fatalf("status %s", trips.byID[id].Status)
	}
}

func TestCompleteTripLocksAStayTrip(t *testing.T) {
	id := uuid.New()
	trips := &stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripStay},
	}}
	svc := NewTripService(trips, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	if err := svc.CompleteTrip(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if trips.byID[id].Status != enum.TripCompleted {
		t.Fatalf("status %s", trips.byID[id].Status)
	}
}

func TestCompleteTripRejectsPlanning(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripPlanning},
	}}, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	err := svc.CompleteTrip(context.Background(), id)
	if !errors.Is(err, repository.ErrTripNotActive) {
		t.Fatalf("got %v", err)
	}
}

func TestCompleteTripRejectsAnAlreadyCompletedTrip(t *testing.T) {
	id := uuid.New()
	svc := NewTripService(&stubTripRecords{byID: map[uuid.UUID]*model.Trip{
		id: {Status: enum.TripCompleted},
	}}, nil, nil, nil, nil, nil, stubAcceptedFriends{})
	err := svc.CompleteTrip(context.Background(), id)
	if !errors.Is(err, repository.ErrTripAlreadyCompleted) {
		t.Fatalf("got %v", err)
	}
}

func TestForkTripRequiresSourceMembership(t *testing.T) {
	caller := &authModel.User{Username: "forker"}
	caller.ID = uuid.New()
	svc := NewTripService(&stubTripRecords{}, nil, stubRoster{}, stubActiveTripRole{err: repository.ErrUserNotTripMember}, nil, nil, stubAcceptedFriends{})
	_, err := svc.ForkTrip(context.Background(), caller, uuid.New(), request.ForkTripRequest{})
	if !errors.Is(err, repository.ErrUserNotTripMember) {
		t.Fatalf("got %v", err)
	}
}
