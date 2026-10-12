package service

import (
	"context"
	"errors"
	"testing"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/friend/model"
	"Road-To-Destination-BE/module/friend/model/response"
	"Road-To-Destination-BE/module/friend/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type memPairs struct {
	rows map[string]*model.Friendship
}

func newMemPairs() *memPairs {
	return &memPairs{rows: map[string]*model.Friendship{}}
}

func (m *memPairs) FindPair(_ context.Context, a, b uuid.UUID) (*model.Friendship, error) {
	return m.rows[pairKey(a, b)], nil
}

func (m *memPairs) Insert(_ context.Context, row *model.Friendship) error {
	key := pairKey(row.UserLowID, row.UserHighID)
	if _, ok := m.rows[key]; ok {
		return repository.ErrRequestAlreadySent
	}
	if row.ID == uuid.Nil {
		row.ID = uuid.New()
	}
	m.rows[key] = row
	return nil
}

func (m *memPairs) Save(_ context.Context, row *model.Friendship) error {
	m.rows[pairKey(row.UserLowID, row.UserHighID)] = row
	return nil
}

func (m *memPairs) Delete(_ context.Context, id uuid.UUID) error {
	for key, row := range m.rows {
		if row.ID == id {
			delete(m.rows, key)
		}
	}
	return nil
}

func pairKey(a, b uuid.UUID) string {
	low, high := model.OrderedPair(a, b)
	return low.String() + high.String()
}

type stubUsers struct {
	byID map[uuid.UUID]*authModel.User
}

func (s stubUsers) FindUserByID(_ context.Context, id uuid.UUID) (*authModel.User, error) {
	user, ok := s.byID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func account(name string) *authModel.User {
	user := &authModel.User{Username: name}
	user.ID = uuid.New()
	return user
}

func TestSendRequestRejectsSelf(t *testing.T) {
	from := account("ada")
	svc := NewFriendService(newMemPairs(), nil, stubUsers{})
	_, err := svc.SendRequest(context.Background(), from, from.ID)
	if !errors.Is(err, repository.ErrCannotFriendSelf) {
		t.Fatalf("got %v", err)
	}
}

func TestSendThenAcceptMakesFriends(t *testing.T) {
	from := account("ada")
	to := account("bea")
	pairs := newMemPairs()
	svc := NewFriendService(pairs, nil, stubUsers{byID: map[uuid.UUID]*authModel.User{to.ID: to, from.ID: from}})

	sent, err := svc.SendRequest(context.Background(), from, to.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Status != enum.FriendStatusPending || sent.UserID != to.ID || sent.Username != "bea" {
		t.Fatalf("sent = %+v", sent)
	}
	_, err = svc.SendRequest(context.Background(), from, to.ID)
	if !errors.Is(err, repository.ErrRequestAlreadySent) {
		t.Fatalf("second send = %v", err)
	}
	_, err = svc.Respond(context.Background(), from.ID, to.ID, true)
	if !errors.Is(err, repository.ErrNotRequestAddressee) {
		t.Fatalf("requester respond = %v", err)
	}
	accepted, err := svc.Respond(context.Background(), to.ID, from.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != enum.FriendStatusAccepted || accepted.Username != "ada" {
		t.Fatalf("accepted = %+v", accepted)
	}
	_, err = svc.SendRequest(context.Background(), from, to.ID)
	if !errors.Is(err, repository.ErrAlreadyFriends) {
		t.Fatalf("already friends = %v", err)
	}
}

func TestReturnRequestAcceptsPendingPair(t *testing.T) {
	from := account("ada")
	to := account("bea")
	svc := NewFriendService(newMemPairs(), nil, stubUsers{byID: map[uuid.UUID]*authModel.User{to.ID: to, from.ID: from}})
	if _, err := svc.SendRequest(context.Background(), from, to.ID); err != nil {
		t.Fatal(err)
	}
	got, err := svc.SendRequest(context.Background(), to, from.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != enum.FriendStatusAccepted {
		t.Fatalf("status = %s", got.Status)
	}
}

func TestRejectedRequestCanBeSentAgain(t *testing.T) {
	from := account("ada")
	to := account("bea")
	pairs := newMemPairs()
	svc := NewFriendService(pairs, nil, stubUsers{byID: map[uuid.UUID]*authModel.User{to.ID: to, from.ID: from}})
	if _, err := svc.SendRequest(context.Background(), from, to.ID); err != nil {
		t.Fatal(err)
	}
	rejected, err := svc.Respond(context.Background(), to.ID, from.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if rejected.Status != enum.FriendStatusRejected {
		t.Fatalf("status = %s", rejected.Status)
	}
	resent, err := svc.SendRequest(context.Background(), to, from.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resent.Status != enum.FriendStatusPending {
		t.Fatalf("status = %s", resent.Status)
	}
	row, _ := pairs.FindPair(context.Background(), from.ID, to.ID)
	if row.RequesterID != to.ID {
		t.Fatalf("requester = %s", row.RequesterID)
	}
}

func TestCancelAndUnfriend(t *testing.T) {
	from := account("ada")
	to := account("bea")
	pairs := newMemPairs()
	svc := NewFriendService(pairs, nil, stubUsers{byID: map[uuid.UUID]*authModel.User{to.ID: to, from.ID: from}})
	if _, err := svc.SendRequest(context.Background(), from, to.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CancelRequest(context.Background(), to.ID, from.ID); !errors.Is(err, repository.ErrRequestNotFound) {
		t.Fatalf("addressee cancel = %v", err)
	}
	if err := svc.CancelRequest(context.Background(), from.ID, to.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Unfriend(context.Background(), from.ID, to.ID); !errors.Is(err, repository.ErrNotFriends) {
		t.Fatalf("unfriend missing = %v", err)
	}
	if _, err := svc.SendRequest(context.Background(), from, to.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Respond(context.Background(), to.ID, from.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := svc.Unfriend(context.Background(), from.ID, to.ID); err != nil {
		t.Fatal(err)
	}
	if len(pairs.rows) != 0 {
		t.Fatalf("rows left = %d", len(pairs.rows))
	}
}

func TestSendRequestUnknownUser(t *testing.T) {
	from := account("ada")
	svc := NewFriendService(newMemPairs(), nil, stubUsers{})
	_, err := svc.SendRequest(context.Background(), from, uuid.New())
	if !errors.Is(err, repository.ErrUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

type memDirectory struct {
	accepted []repository.FriendRow
	pending  []repository.FriendRow
	cands    []repository.UserCandidate
	rels     map[uuid.UUID]model.Friendship
}

func (m memDirectory) ListAccepted(context.Context, uuid.UUID) ([]repository.FriendRow, error) {
	return m.accepted, nil
}

func (m memDirectory) ListPending(context.Context, uuid.UUID, bool) ([]repository.FriendRow, error) {
	return m.pending, nil
}

func (m memDirectory) SearchCandidates(context.Context, uuid.UUID, string, string) ([]repository.UserCandidate, error) {
	return m.cands, nil
}

func (m memDirectory) Relations(context.Context, uuid.UUID, []uuid.UUID) (map[uuid.UUID]model.Friendship, error) {
	if m.rels == nil {
		return map[uuid.UUID]model.Friendship{}, nil
	}
	return m.rels, nil
}

func TestListFriendsFiltersAndCounts(t *testing.T) {
	ada := uuid.New()
	bea := uuid.New()
	cy := uuid.New()
	dir := memDirectory{accepted: []repository.FriendRow{
		{UserID: bea, Username: "Bea"},
		{UserID: cy, Username: "cypress"},
		{UserID: ada, Username: "adam"},
	}}
	svc := NewFriendService(nil, dir, nil)
	all, err := svc.ListFriends(context.Background(), uuid.New(), " ")
	if err != nil {
		t.Fatal(err)
	}
	if all.Count != 3 || all.Friends[0].Username != "adam" {
		t.Fatalf("all = %+v", all)
	}
	filtered, err := svc.ListFriends(context.Background(), uuid.New(), "Be")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Count != 1 || filtered.Friends[0].Username != "Bea" {
		t.Fatalf("filtered = %+v", filtered)
	}
}

func TestSearchRanksPhoneThenUsernameAndCapsAtTwenty(t *testing.T) {
	viewer := uuid.New()
	phoneUser := uuid.New()
	exact := uuid.New()
	prefix := uuid.New()
	contains := uuid.New()
	cands := []repository.UserCandidate{
		{UserID: contains, Username: "xxannxx"},
		{UserID: prefix, Username: "annette"},
		{UserID: exact, Username: "ann"},
		{UserID: phoneUser, Username: "mobile", PhoneMatched: true},
	}
	friendRow := model.Friendship{RequesterID: viewer, Status: enum.FriendStatusAccepted}
	friendRow.ID = uuid.New()
	outRow := model.Friendship{RequesterID: viewer, Status: enum.FriendStatusPending}
	outRow.ID = uuid.New()
	dir := memDirectory{
		cands: cands,
		rels: map[uuid.UUID]model.Friendship{
			exact:     friendRow,
			prefix:    outRow,
			phoneUser: model.Friendship{},
		},
	}
	svc := NewFriendService(nil, dir, nil)
	got, err := svc.Search(context.Background(), viewer, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Users) != 4 {
		t.Fatalf("len = %d", len(got.Users))
	}
	if got.Users[0].UserID != phoneUser || got.Users[0].Relation != response.RelationNone {
		t.Fatalf("first = %+v", got.Users[0])
	}
	if got.Users[1].UserID != exact || got.Users[1].Relation != response.RelationFriends {
		t.Fatalf("second = %+v", got.Users[1])
	}
	if got.Users[2].UserID != prefix || got.Users[2].Relation != response.RelationOutgoing {
		t.Fatalf("third = %+v", got.Users[2])
	}
	if got.Users[3].UserID != contains || got.Users[3].Relation != response.RelationNone {
		t.Fatalf("fourth = %+v", got.Users[3])
	}
}

func TestSearchCapsAtTwenty(t *testing.T) {
	cands := make([]repository.UserCandidate, 0, 25)
	for i := 0; i < 25; i++ {
		cands = append(cands, repository.UserCandidate{
			UserID:   uuid.New(),
			Username: "annz",
		})
	}
	svc := NewFriendService(nil, memDirectory{cands: cands}, nil)
	got, err := svc.Search(context.Background(), uuid.New(), "ann")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Users) != searchLimit {
		t.Fatalf("len = %d", len(got.Users))
	}
}

func TestSearchRequiresQuery(t *testing.T) {
	svc := NewFriendService(nil, memDirectory{}, nil)
	_, err := svc.Search(context.Background(), uuid.New(), "  ")
	if !errors.Is(err, repository.ErrEmptySearch) {
		t.Fatalf("got %v", err)
	}
}
