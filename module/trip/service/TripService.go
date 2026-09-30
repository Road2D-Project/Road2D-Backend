package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/model/response"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	tripJoinPathPrefix  = "/v1/trips/join/"
	publicTripListLimit = 50
)

// TripRecordRepository persists trips and their membership rows.
type TripRecordRepository interface {
	CreateTripWithMembers(ctx context.Context, trip *model.Trip, members []model.TripMember, main *model.InitialMainBranch) error
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	UpdateTripInfo(ctx context.Context, id uuid.UUID, updates map[string]any) error
	UpdateInviteToken(ctx context.Context, id uuid.UUID, token string) error
	DeleteTrip(ctx context.Context, id uuid.UUID) error
	ListActiveTripByUser(ctx context.Context, userID uuid.UUID) ([]repository.ActiveTripByUser, error)
	ListPublicTrips(ctx context.Context, limit int) ([]model.Trip, error)
}

// UserByIDFinder loads accounts named on a create or fork roster.
type UserByIDFinder interface {
	FindUserByID(ctx context.Context, id uuid.UUID) (*authModel.User, error)
}

// TripRoster lists the active seats copied by a fork.
type TripRoster interface {
	ListActiveMembers(ctx context.Context, tripId uuid.UUID) ([]model.TripMember, error)
}

// ActiveTripMemberRoleReader loads leader/admin/member only when status is active.
type ActiveTripMemberRoleReader interface {
	FindTripActiveMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) (enum.TripRole, error)
}

// TripMemberRoleCache is the Redis copy of an active member's trip role.
type TripMemberRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

// LocationByIDFinder loads a catalog place named on a reviewed main branch.
type LocationByIDFinder interface {
	FindLocationById(ctx context.Context, locationId uuid.UUID) (*model.Location, error)
}

type TripService struct {
	trips       TripRecordRepository
	users       UserByIDFinder
	roster      TripRoster
	activeRoles ActiveTripMemberRoleReader
	roleCache   TripMemberRoleCache
	locations   LocationByIDFinder
}

func NewTripService(
	trips TripRecordRepository,
	users UserByIDFinder,
	roster TripRoster,
	activeRoles ActiveTripMemberRoleReader,
	roleCache TripMemberRoleCache,
	locations LocationByIDFinder,
) *TripService {
	return &TripService{
		trips:       trips,
		users:       users,
		roster:      roster,
		activeRoles: activeRoles,
		roleCache:   roleCache,
		locations:   locations,
	}
}

// CreateTrip inserts a planning trip. The caller is leader. memberUserIds are
// seated immediately as members. A matching group chat is not created yet;
// that roster must stay aligned with these members once chat exists.
func (s *TripService) CreateTrip(ctx context.Context, owner *authModel.User, req request.CreateTripRequest) (*response.TripResponse, error) {
	if owner == nil {
		return nil, errors.New("missing current user")
	}
	if req.TripType == nil {
		return nil, repository.ErrTripTypePolicyUnset
	}
	name := utils.Santize(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	limit, err := memberLimitFor(*req.TripType)
	if err != nil {
		return nil, err
	}
	memberIDs, err := otherMemberIDs(owner.ID, req.MemberUserIDs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	members := []model.TripMember{{
		UserID:   owner.ID,
		Role:     enum.TripRoleLeader,
		Status:   enum.MembershipActive,
		Nickname: owner.Username,
		JoinedAt: &now,
	}}
	for _, id := range memberIDs {
		user, err := s.lookupUser(ctx, id)
		if err != nil {
			return nil, err
		}
		joined := now
		members = append(members, model.TripMember{
			UserID:   user.ID,
			Role:     enum.TripRoleMember,
			Status:   enum.MembershipActive,
			Nickname: user.Username,
			JoinedAt: &joined,
		})
	}
	if err := seatsFit(len(members), limit); err != nil {
		return nil, err
	}
	main, err := s.initialMainBranch(ctx, req.MainBranch)
	if err != nil {
		return nil, err
	}
	trip := &model.Trip{
		OwnerID:     owner.ID,
		Name:        name,
		Status:      enum.TripPlanning,
		TripType:    *req.TripType,
		MemberLimit: limit,
		Visibility:  req.Visibility,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Note:        utils.Santize(req.Note),
		InviteToken: uuid.New().String(),
	}
	if err := s.trips.CreateTripWithMembers(ctx, trip, members, main); err != nil {
		return nil, err
	}
	for _, member := range members {
		s.cacheTripMemberRole(ctx, trip.ID, member.UserID, member.Role)
	}
	return tripResponseWithRole(trip, enum.TripRoleLeader), nil
}

// ForkTrip opens a new trip owned by the caller. Active members of the source
// are copied as members, minus excludeUserIds. The route graph is not copied.
func (s *TripService) ForkTrip(ctx context.Context, caller *authModel.User, sourceID uuid.UUID, req request.ForkTripRequest) (*response.TripResponse, error) {
	if caller == nil {
		return nil, errors.New("missing current user")
	}
	if _, err := s.activeRoles.FindTripActiveMemberRole(ctx, sourceID, caller.ID); err != nil {
		return nil, err
	}
	source, err := s.trips.FindTripByID(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	tripType := source.TripType
	if req.TripType != nil {
		tripType = *req.TripType
	}
	limit, err := memberLimitFor(tripType)
	if err != nil {
		return nil, err
	}
	roster, err := s.roster.ListActiveMembers(ctx, sourceID)
	if err != nil {
		return nil, err
	}
	excluded := map[uuid.UUID]struct{}{}
	for _, id := range req.ExcludeUserIDs {
		if id == caller.ID {
			continue
		}
		excluded[id] = struct{}{}
	}
	now := time.Now().UTC()
	members := []model.TripMember{{
		UserID:   caller.ID,
		Role:     enum.TripRoleLeader,
		Status:   enum.MembershipActive,
		Nickname: caller.Username,
		JoinedAt: &now,
	}}
	for i := range roster {
		sourceMember := roster[i]
		if sourceMember.UserID == caller.ID {
			continue
		}
		if _, drop := excluded[sourceMember.UserID]; drop {
			continue
		}
		if !sourceMember.Status.IsActive() {
			continue
		}
		nickname := sourceMember.Nickname
		if nickname == "" && sourceMember.User != nil {
			nickname = sourceMember.User.Username
		}
		joined := now
		members = append(members, model.TripMember{
			UserID:   sourceMember.UserID,
			Role:     enum.TripRoleMember,
			Status:   enum.MembershipActive,
			Nickname: nickname,
			JoinedAt: &joined,
		})
	}
	if err := seatsFit(len(members), limit); err != nil {
		return nil, err
	}
	name := source.Name
	if req.Name != nil {
		name = utils.Santize(*req.Name)
		if name == "" {
			return nil, errors.New("name is required")
		}
	}
	note := source.Note
	if req.Note != nil {
		note = utils.Santize(*req.Note)
	}
	visibility := source.Visibility
	if req.Visibility != nil {
		visibility = *req.Visibility
	}
	start := source.StartTime
	if req.StartTime != nil {
		start = req.StartTime
	}
	end := source.EndTime
	if req.EndTime != nil {
		end = req.EndTime
	}
	trip := &model.Trip{
		OwnerID:     caller.ID,
		Name:        name,
		Status:      enum.TripPlanning,
		TripType:    tripType,
		MemberLimit: limit,
		Visibility:  visibility,
		StartTime:   start,
		EndTime:     end,
		Note:        note,
		InviteToken: uuid.New().String(),
	}
	if err := s.trips.CreateTripWithMembers(ctx, trip, members, nil); err != nil {
		return nil, err
	}
	for _, member := range members {
		s.cacheTripMemberRole(ctx, trip.ID, member.UserID, member.Role)
	}
	return tripResponseWithRole(trip, enum.TripRoleLeader), nil
}

// GetTrip loads the trip by id. A private trip is visible only to an active member.
func (s *TripService) GetTrip(ctx context.Context, tripID, userID uuid.UUID) (*response.TripResponse, error) {
	trip, err := s.trips.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if s.activeRoles == nil {
		return response.FromTripPtr(trip, nil), nil
	}
	role, err := s.activeRoles.FindTripActiveMemberRole(ctx, tripID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotTripMember) {
			if !trip.Visibility {
				return nil, repository.ErrTripNotFound
			}
			return response.FromTripPtr(trip, nil), nil
		}
		return nil, err
	}
	return tripResponseWithRole(trip, role), nil
}

// ListActiveTripsForUser returns trips where the caller currently has an active seat.
func (s *TripService) ListActiveTripsForUser(ctx context.Context, userID uuid.UUID) (*response.TripListResponse, error) {
	rows, err := s.trips.ListActiveTripByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]response.TripResponse, 0, len(rows))
	for i := range rows {
		role := rows[i].Role
		items = append(items, response.FromTrip(&rows[i].Trip, &role))
	}
	return &response.TripListResponse{Trips: items}, nil
}

// ListPublicTrips returns trips outsiders can find. Private trips are omitted.
func (s *TripService) ListPublicTrips(ctx context.Context) (*response.TripListResponse, error) {
	rows, err := s.trips.ListPublicTrips(ctx, publicTripListLimit)
	if err != nil {
		return nil, err
	}
	items := make([]response.TripResponse, 0, len(rows))
	for i := range rows {
		items = append(items, response.FromTrip(&rows[i], nil))
	}
	return &response.TripListResponse{Trips: items}, nil
}

// UpdateTrip writes the provided fields. TripType and MemberLimit are not accepted.
func (s *TripService) UpdateTrip(ctx context.Context, tripID uuid.UUID, req request.UpdateTripRequest, myRole enum.TripRole) (*response.TripResponse, error) {
	updates := map[string]any{}
	if req.Name != nil {
		cleaned := utils.Santize(*req.Name)
		if cleaned == "" {
			return nil, errors.New("name is required")
		}
		updates["name"] = cleaned
	}
	if req.Note != nil {
		updates["note"] = utils.Santize(*req.Note)
	}
	if req.StartTime != nil {
		updates["start_time"] = *req.StartTime
	}
	if req.EndTime != nil {
		updates["end_time"] = *req.EndTime
	}
	if req.Visibility != nil {
		updates["visibility"] = *req.Visibility
	}
	if len(updates) == 0 {
		return nil, repository.ErrNoTripUpdate
	}
	if err := s.trips.UpdateTripInfo(ctx, tripID, updates); err != nil {
		return nil, err
	}
	trip, err := s.trips.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	return tripResponseWithRole(trip, myRole), nil
}

// DeleteTrip removes the trip; members cascade with the row.
func (s *TripService) DeleteTrip(ctx context.Context, tripID uuid.UUID) error {
	return s.trips.DeleteTrip(ctx, tripID)
}

// CreateInviteLink returns the current token, or mints a new one when rotate is set / none exists.
func (s *TripService) CreateInviteLink(ctx context.Context, tripID uuid.UUID, rotate bool) (*response.TripInviteLinkResponse, error) {
	trip, err := s.trips.FindTripByID(ctx, tripID)
	if err != nil {
		return nil, err
	}
	token := trip.InviteToken
	if rotate || token == "" {
		token = uuid.New().String()
		if err := s.trips.UpdateInviteToken(ctx, tripID, token); err != nil {
			return nil, err
		}
	}
	return &response.TripInviteLinkResponse{
		Token:    token,
		JoinPath: tripJoinPathPrefix + token,
	}, nil
}

func (s *TripService) lookupUser(ctx context.Context, id uuid.UUID) (*authModel.User, error) {
	if s.users == nil {
		return nil, repository.ErrUserNotFound
	}
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

func otherMemberIDs(ownerID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil || id == ownerID {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, repository.ErrTripMembersRequired
	}
	return out, nil
}

func (s *TripService) cacheTripMemberRole(ctx context.Context, tripID, userID uuid.UUID, role enum.TripRole) {
	if s.roleCache == nil {
		return
	}
	_ = s.roleCache.SetCachedTripMemberRole(ctx, tripID, userID, role)
}

func tripResponseWithRole(trip *model.Trip, role enum.TripRole) *response.TripResponse {
	out := response.FromTrip(trip, response.RolePtr(role))
	return &out
}
