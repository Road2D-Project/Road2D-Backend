package service

import (
	"context"
	"errors"
	"sort"
	"strings"

	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/friend/model"
	"Road-To-Destination-BE/module/friend/model/response"
	"Road-To-Destination-BE/module/friend/repository"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// FriendPairStore reads and writes one friendship row.
type FriendPairStore interface {
	FindPair(ctx context.Context, a, b uuid.UUID) (*model.Friendship, error)
	Insert(ctx context.Context, row *model.Friendship) error
	Save(ctx context.Context, row *model.Friendship) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// FriendDirectory lists friends and searches accounts.
type FriendDirectory interface {
	ListAccepted(ctx context.Context, userID uuid.UUID) ([]repository.FriendRow, error)
	ListPending(ctx context.Context, userID uuid.UUID, incoming bool) ([]repository.FriendRow, error)
	SearchCandidates(ctx context.Context, viewer uuid.UUID, usernameLike, exactPhone string) ([]repository.UserCandidate, error)
	Relations(ctx context.Context, viewer uuid.UUID, others []uuid.UUID) (map[uuid.UUID]model.Friendship, error)
}

// UserByIDFinder loads the account named on a request.
type UserByIDFinder interface {
	FindUserByID(ctx context.Context, id uuid.UUID) (*authModel.User, error)
}

type FriendService struct {
	pairs     FriendPairStore
	directory FriendDirectory
	users     UserByIDFinder
}

func NewFriendService(pairs FriendPairStore, directory FriendDirectory, users UserByIDFinder) *FriendService {
	return &FriendService{pairs: pairs, directory: directory, users: users}
}

// SendRequest opens a pending row, accepts it when the other user already
// asked, or reopens a rejected row.
func (s *FriendService) SendRequest(ctx context.Context, from *authModel.User, toID uuid.UUID) (*response.FriendRequestResponse, error) {
	if from == nil {
		return nil, errors.New("missing current user")
	}
	if from.ID == toID {
		return nil, repository.ErrCannotFriendSelf
	}
	target, err := s.lookupUser(ctx, toID)
	if err != nil {
		return nil, err
	}
	row, err := s.pairs.FindPair(ctx, from.ID, toID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		low, high := model.OrderedPair(from.ID, toID)
		created := &model.Friendship{
			UserLowID:   low,
			UserHighID:  high,
			RequesterID: from.ID,
			Status:      enum.FriendStatusPending,
		}
		if err := s.pairs.Insert(ctx, created); err != nil {
			return nil, err
		}
		return requestResponse(target, enum.FriendStatusPending), nil
	}
	switch {
	case row.Status.IsAccepted():
		return nil, repository.ErrAlreadyFriends
	case row.Status.IsPending() && row.RequesterID == from.ID:
		return nil, repository.ErrRequestAlreadySent
	case row.Status.IsPending():
		row.Status = enum.FriendStatusAccepted
		if err := s.pairs.Save(ctx, row); err != nil {
			return nil, err
		}
		return requestResponse(target, enum.FriendStatusAccepted), nil
	default:
		row.Status = enum.FriendStatusPending
		row.RequesterID = from.ID
		if err := s.pairs.Save(ctx, row); err != nil {
			return nil, err
		}
		return requestResponse(target, enum.FriendStatusPending), nil
	}
}

// Respond accepts or rejects a pending request addressed to userID.
func (s *FriendService) Respond(ctx context.Context, userID, otherID uuid.UUID, accept bool) (*response.FriendRequestResponse, error) {
	if userID == otherID {
		return nil, repository.ErrCannotFriendSelf
	}
	row, err := s.pairs.FindPair(ctx, userID, otherID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, repository.ErrRequestNotFound
	}
	if row.Status.IsAccepted() {
		return nil, repository.ErrAlreadyFriends
	}
	if !row.Status.IsPending() || row.RequesterID == userID {
		if row.RequesterID == userID && row.Status.IsPending() {
			return nil, repository.ErrNotRequestAddressee
		}
		return nil, repository.ErrRequestNotFound
	}
	if accept {
		row.Status = enum.FriendStatusAccepted
	} else {
		row.Status = enum.FriendStatusRejected
	}
	if err := s.pairs.Save(ctx, row); err != nil {
		return nil, err
	}
	other, err := s.lookupUser(ctx, otherID)
	if err != nil {
		return nil, err
	}
	return requestResponse(other, row.Status), nil
}

// CancelRequest deletes a pending request the caller sent.
func (s *FriendService) CancelRequest(ctx context.Context, userID, otherID uuid.UUID) error {
	row, err := s.pairs.FindPair(ctx, userID, otherID)
	if err != nil {
		return err
	}
	if row == nil || !row.Status.IsPending() || row.RequesterID != userID {
		return repository.ErrRequestNotFound
	}
	return s.pairs.Delete(ctx, row.ID)
}

// Unfriend deletes an accepted friendship.
func (s *FriendService) Unfriend(ctx context.Context, userID, otherID uuid.UUID) error {
	if userID == otherID {
		return repository.ErrCannotFriendSelf
	}
	row, err := s.pairs.FindPair(ctx, userID, otherID)
	if err != nil {
		return err
	}
	if row == nil || !row.Status.IsAccepted() {
		return repository.ErrNotFriends
	}
	return s.pairs.Delete(ctx, row.ID)
}

// ListFriends returns accepted friends. query filters usernames with the same
// substring rank as search. The filter runs in memory over this user's friends.
// Upgrade later: push that rank into SQL when friend lists outgrow a single page.
func (s *FriendService) ListFriends(ctx context.Context, userID uuid.UUID, rawQuery string) (*response.FriendListResponse, error) {
	rows, err := s.directory.ListAccepted(ctx, userID)
	if err != nil {
		return nil, err
	}
	query := normalizeSearchQuery(rawQuery)
	type ranked struct {
		row   repository.FriendRow
		score int
	}
	matched := make([]ranked, 0, len(rows))
	for _, row := range rows {
		score := usernameExact
		if query != "" {
			var ok bool
			score, ok = rankUsername(query, row.Username)
			if !ok {
				continue
			}
		}
		matched = append(matched, ranked{row: row, score: score})
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score < matched[j].score
		}
		return strings.ToLower(matched[i].row.Username) < strings.ToLower(matched[j].row.Username)
	})
	friends := make([]response.FriendResponse, 0, len(matched))
	for _, item := range matched {
		friends = append(friends, response.FriendResponse{
			UserID:   item.row.UserID,
			Username: item.row.Username,
		})
	}
	return &response.FriendListResponse{Count: len(friends), Friends: friends}, nil
}

func (s *FriendService) ListRequests(ctx context.Context, userID uuid.UUID, incoming bool) (*response.FriendRequestListResponse, error) {
	rows, err := s.directory.ListPending(ctx, userID, incoming)
	if err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		return strings.ToLower(rows[i].Username) < strings.ToLower(rows[j].Username)
	})
	requests := make([]response.FriendRequestResponse, 0, len(rows))
	for _, row := range rows {
		requests = append(requests, response.FriendRequestResponse{
			UserID:   row.UserID,
			Username: row.Username,
			Status:   row.Status,
		})
	}
	return &response.FriendRequestListResponse{Requests: requests}, nil
}

// Search ranks username substring hits and exact full-number phone hits.
// See rankUsername and exactPhoneQuery for the upgrades still pending.
func (s *FriendService) Search(ctx context.Context, viewer uuid.UUID, raw string) (*response.FriendSearchResponse, error) {
	query := normalizeSearchQuery(raw)
	if query == "" {
		return nil, repository.ErrEmptySearch
	}
	if len(query) > maxSearchQueryLen {
		return nil, repository.ErrSearchQueryTooLong
	}
	candidates, err := s.directory.SearchCandidates(ctx, viewer, likeContains(query), exactPhoneQuery(raw))
	if err != nil {
		return nil, err
	}
	type ranked struct {
		hit   response.FriendSearchHit
		score int
	}
	ids := make([]uuid.UUID, 0, len(candidates))
	matched := make([]ranked, 0, len(candidates))
	for _, candidate := range candidates {
		score, ok := rankUsername(query, candidate.Username)
		if candidate.PhoneMatched {
			score = phoneExact
			ok = true
		}
		if !ok {
			continue
		}
		ids = append(ids, candidate.UserID)
		matched = append(matched, ranked{
			score: score,
			hit: response.FriendSearchHit{
				UserID:   candidate.UserID,
				Username: candidate.Username,
				Relation: response.RelationNone,
			},
		})
	}
	relations, err := s.directory.Relations(ctx, viewer, ids)
	if err != nil {
		return nil, err
	}
	for i := range matched {
		matched[i].hit.Relation = relationOf(viewer, relations[matched[i].hit.UserID])
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].score != matched[j].score {
			return matched[i].score < matched[j].score
		}
		return strings.ToLower(matched[i].hit.Username) < strings.ToLower(matched[j].hit.Username)
	})
	if len(matched) > searchLimit {
		matched = matched[:searchLimit]
	}
	users := make([]response.FriendSearchHit, 0, len(matched))
	for _, item := range matched {
		users = append(users, item.hit)
	}
	return &response.FriendSearchResponse{Users: users}, nil
}

func (s *FriendService) lookupUser(ctx context.Context, id uuid.UUID) (*authModel.User, error) {
	user, err := s.users.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, repository.ErrUserNotFound
		}
		return nil, err
	}
	if user == nil {
		return nil, repository.ErrUserNotFound
	}
	return user, nil
}

func requestResponse(user *authModel.User, status enum.FriendStatus) *response.FriendRequestResponse {
	return &response.FriendRequestResponse{
		UserID:   user.ID,
		Username: user.Username,
		Status:   status,
	}
}

func relationOf(viewer uuid.UUID, row model.Friendship) response.FriendRelation {
	if row.ID == uuid.Nil && row.Status == 0 && row.RequesterID == uuid.Nil {
		return response.RelationNone
	}
	switch {
	case row.Status.IsAccepted():
		return response.RelationFriends
	case row.Status.IsPending() && row.RequesterID == viewer:
		return response.RelationOutgoing
	case row.Status.IsPending():
		return response.RelationIncoming
	default:
		return response.RelationNone
	}
}
