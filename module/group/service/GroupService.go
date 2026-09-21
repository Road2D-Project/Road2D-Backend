package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/request"
	"Road-To-Destination-BE/module/group/model/response"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GroupRecordRepository persists groups and their membership rows.
type GroupRecordRepository interface {
	CreateGroupWithMembers(ctx context.Context, group *model.Group, members []model.GroupMember) error
	FindGroupByID(ctx context.Context, id uuid.UUID) (*model.Group, error)
	UpdateGroupInfo(ctx context.Context, id uuid.UUID, name, description *string) error
	DeleteGroup(ctx context.Context, id uuid.UUID) error
	ListActiveGroupByUser(ctx context.Context, userID uuid.UUID) ([]repository.ActiveGroupByUser, error)
	CountGroupTrips(ctx context.Context, groupID uuid.UUID) (int64, error)
}

// UserByUsernameFinder loads an account by login name when adding admins.
type UserByUsernameFinder interface {
	FindUserByUsername(ctx context.Context, username string) (*authModel.User, error)
}

// GroupMemberRoleCache is the Redis copy of an active member's role.
type GroupMemberRoleCache interface {
	SetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID, role enum.GroupRole) error
}

// ActiveGroupMemberRoleReader loads owner/admin/member only when status is active.
type ActiveGroupMemberRoleReader interface {
	FindGroupActiveMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error)
}

type GroupService struct {
	groupRecords GroupRecordRepository
	usersByName  UserByUsernameFinder
	activeRoles  ActiveGroupMemberRoleReader
	roleCache    GroupMemberRoleCache
}

func NewGroupService(
	groupRecords GroupRecordRepository,
	usersByName UserByUsernameFinder,
	activeRoles ActiveGroupMemberRoleReader,
	roleCache GroupMemberRoleCache,
) *GroupService {
	return &GroupService{
		groupRecords: groupRecords,
		usersByName:  usersByName,
		activeRoles:  activeRoles,
		roleCache:    roleCache,
	}
}

func (s *GroupService) CreateGroup(ctx context.Context, owner *authModel.User, req request.CreateGroupRequest) (*response.GroupResponse, error) {
	if owner == nil {
		return nil, errors.New("missing current user")
	}
	name := utils.Santize(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	now := time.Now().UTC()
	group := &model.Group{
		Name:        name,
		Description: utils.Santize(req.Description),
		OwnerID:     owner.ID,
	}
	members := []model.GroupMember{{
		UserID:   owner.ID,
		Role:     enum.GroupRoleOwner,
		Status:   enum.MembershipActive,
		Nickname: owner.Username,
		JoinedAt: &now,
	}}
	seen := map[string]struct{}{owner.Username: {}}
	for _, raw := range req.AdminUsernames {
		username := utils.Santize(raw)
		if username == "" {
			continue
		}
		if _, dup := seen[username]; dup {
			continue
		}
		seen[username] = struct{}{}
		admin, err := s.usersByName.FindUserByUsername(ctx, username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, repository.ErrAdminUserNotFound
			}
			return nil, err
		}
		if admin.ID == owner.ID {
			continue
		}
		members = append(members, model.GroupMember{
			UserID:      admin.ID,
			Role:        enum.GroupRoleAdmin,
			Status:      enum.MembershipActive,
			Nickname:    admin.Username,
			JoinedAt:    &now,
			InvitorName: new(owner.Username),
		})
	}
	if err := s.groupRecords.CreateGroupWithMembers(ctx, group, members); err != nil {
		return nil, err
	}
	s.cacheGroupMemberRoles(ctx, group.ID, members)
	return groupResponseWithRole(group, enum.GroupRoleOwner), nil
}

func (s *GroupService) GetGroup(ctx context.Context, groupID, userID uuid.UUID) (*response.GroupResponse, error) {
	group, err := s.groupRecords.FindGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if s.activeRoles == nil {
		return response.FromGroupPtr(group, nil), nil
	}
	role, err := s.activeRoles.FindGroupActiveMemberRole(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrUserNotGroupMember) {
			return response.FromGroupPtr(group, nil), nil
		}
		return nil, err
	}
	return groupResponseWithRole(group, role), nil
}

func (s *GroupService) ListActiveGroupsForUser(ctx context.Context, userID uuid.UUID) (*response.GroupListResponse, error) {
	rows, err := s.groupRecords.ListActiveGroupByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]response.GroupResponse, 0, len(rows))
	for i := range rows {
		role := rows[i].Role
		items = append(items, response.FromGroup(&rows[i].Group, &role))
	}
	return &response.GroupListResponse{Groups: items}, nil
}

func (s *GroupService) UpdateGroup(ctx context.Context, groupID uuid.UUID, req request.UpdateGroupRequest, myRole enum.GroupRole) (*response.GroupResponse, error) {
	var name, description *string
	if req.Name != nil {
		cleaned := utils.Santize(*req.Name)
		if cleaned == "" {
			return nil, errors.New("name is required")
		}
		name = &cleaned
	}
	if req.Description != nil {
		cleaned := utils.Santize(*req.Description)
		description = &cleaned
	}
	if name == nil && description == nil {
		return nil, repository.ErrNoGroupUpdate
	}
	if err := s.groupRecords.UpdateGroupInfo(ctx, groupID, name, description); err != nil {
		return nil, err
	}
	group, err := s.groupRecords.FindGroupByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return groupResponseWithRole(group, myRole), nil
}

func (s *GroupService) DeleteGroup(ctx context.Context, groupID uuid.UUID) error {
	n, err := s.groupRecords.CountGroupTrips(ctx, groupID)
	if err != nil {
		return err
	}
	if n > 0 {
		return repository.ErrGroupHasTrips
	}
	return s.groupRecords.DeleteGroup(ctx, groupID)
}

func (s *GroupService) cacheGroupMemberRoles(ctx context.Context, groupID uuid.UUID, members []model.GroupMember) {
	if s.roleCache == nil {
		return
	}
	for _, m := range members {
		_ = s.roleCache.SetCachedGroupMemberRole(ctx, groupID, m.UserID, m.Role)
	}
}

func groupResponseWithRole(group *model.Group, role enum.GroupRole) *response.GroupResponse {
	out := response.FromGroup(group, response.RolePtr(role))
	return &out
}
