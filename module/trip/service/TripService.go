package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	groupModel "Road-To-Destination-BE/module/group/model"
	groupRepo "Road-To-Destination-BE/module/group/repository"
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
)

const tripJoinPathPrefix = "/v1/trips/join/"

// TripRecordRepository persists trips and their membership rows.
type TripRecordRepository interface {
	CreateTripWithMembers(ctx context.Context, trip *model.Trip, members []model.TripMember) error
	FindTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	UpdateTripInfo(ctx context.Context, id uuid.UUID, updates map[string]any) error
	UpdateInviteToken(ctx context.Context, id uuid.UUID, token string) error
	DeleteTrip(ctx context.Context, id uuid.UUID) error
	ListActiveTripByUser(ctx context.Context, userID uuid.UUID) ([]repository.ActiveTripByUser, error)
}

// GroupByIDFinder loads the standing group a trip is created under.
type GroupByIDFinder interface {
	FindGroupByID(ctx context.Context, id uuid.UUID) (*groupModel.Group, error)
}

// ActiveGroupMemberRoleReader confirms the caller is an active group member.
type ActiveGroupMemberRoleReader interface {
	FindGroupActiveMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error)
}

// ActiveTripMemberRoleReader loads leader/member only when status is active.
type ActiveTripMemberRoleReader interface {
	FindTripActiveMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) (enum.TripRole, error)
}

// TripMemberRoleCache is the Redis copy of an active member's trip role.
type TripMemberRoleCache interface {
	SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error
}

type TripService struct {
	trips       TripRecordRepository
	groups      GroupByIDFinder
	groupRoles  ActiveGroupMemberRoleReader
	activeRoles ActiveTripMemberRoleReader
	roleCache   TripMemberRoleCache
}

func NewTripService(
	trips TripRecordRepository,
	groups GroupByIDFinder,
	groupRoles ActiveGroupMemberRoleReader,
	activeRoles ActiveTripMemberRoleReader,
	roleCache TripMemberRoleCache,
) *TripService {
	return &TripService{
		trips:       trips,
		groups:      groups,
		groupRoles:  groupRoles,
		activeRoles: activeRoles,
		roleCache:   roleCache,
	}
}

// CreateTrip checks the caller is an active member of the given group, then
// inserts a planning trip plus a leader membership row and caches that role.
func (s *TripService) CreateTrip(ctx context.Context, owner *authModel.User, req request.CreateTripRequest) (*response.TripResponse, error) {
	if owner == nil {
		return nil, errors.New("missing current user")
	}
	name := utils.Santize(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if _, err := s.groups.FindGroupByID(ctx, req.GroupID); err != nil {
		if errors.Is(err, groupRepo.ErrGroupNotFound) {
			return nil, repository.ErrGroupNotFound
		}
		return nil, err
	}
	if _, err := s.groupRoles.FindGroupActiveMemberRole(ctx, req.GroupID, owner.ID); err != nil {
		if errors.Is(err, groupRepo.ErrUserNotGroupMember) {
			return nil, repository.ErrUserNotGroupMember
		}
		return nil, err
	}
	now := time.Now().UTC()
	groupID := req.GroupID
	trip := &model.Trip{
		GroupID:     &groupID,
		OwnerID:     owner.ID,
		Name:        name,
		Status:      enum.TripPlanning,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Note:        utils.Santize(req.Note),
		InviteToken: uuid.New().String(),
	}
	members := []model.TripMember{{
		UserID:   owner.ID,
		Role:     enum.TripRoleLeader,
		Status:   enum.MembershipActive,
		Nickname: owner.Username,
		JoinedAt: &now,
	}}
	if err := s.trips.CreateTripWithMembers(ctx, trip, members); err != nil {
		return nil, err
	}
	s.cacheTripMemberRole(ctx, trip.ID, owner.ID, enum.TripRoleLeader)
	return tripResponseWithRole(trip, enum.TripRoleLeader), nil
}

// GetTrip loads the trip by id and fills myRole when the caller is an active member.
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

// UpdateTrip sanitizes the provided fields, writes them, and returns the trip with the caller's role.
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
