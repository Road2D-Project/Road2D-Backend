package service

import (
	authModel "Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/model/request"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type stubGroupRecords struct {
	created   *model.Group
	members   []model.GroupMember
	byID      map[uuid.UUID]*model.Group
	listed    []repository.ActiveGroupByUser
	tripCount int64
	updateErr error
	deleted   uuid.UUID
}

func (s *stubGroupRecords) CreateGroupWithMembers(_ context.Context, group *model.Group, members []model.GroupMember) error {
	if group.ID == uuid.Nil {
		group.ID = uuid.New()
	}
	copied := *group
	s.created = &copied
	s.members = append([]model.GroupMember(nil), members...)
	if s.byID == nil {
		s.byID = map[uuid.UUID]*model.Group{}
	}
	s.byID[group.ID] = &copied
	return nil
}

func (s *stubGroupRecords) FindGroupByID(_ context.Context, id uuid.UUID) (*model.Group, error) {
	g, ok := s.byID[id]
	if !ok {
		return nil, repository.ErrGroupNotFound
	}
	copied := *g
	return &copied, nil
}

func (s *stubGroupRecords) UpdateGroupInfo(_ context.Context, id uuid.UUID, name, description, policy *string) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	g, ok := s.byID[id]
	if !ok {
		return repository.ErrGroupNotFound
	}
	if name != nil {
		g.Name = *name
	}
	if description != nil {
		g.Description = *description
	}
	if policy != nil {
		g.Policy = *policy
	}
	return nil
}

func (s *stubGroupRecords) DeleteGroup(_ context.Context, id uuid.UUID) error {
	s.deleted = id
	return nil
}

func (s *stubGroupRecords) ListActiveGroupByUser(context.Context, uuid.UUID) ([]repository.ActiveGroupByUser, error) {
	return s.listed, nil
}

func (s *stubGroupRecords) CountGroupTrips(context.Context, uuid.UUID) (int64, error) {
	return s.tripCount, nil
}

type stubUsersByName struct {
	byName map[string]*authModel.User
}

func (s stubUsersByName) FindUserByUsername(_ context.Context, username string) (*authModel.User, error) {
	u, ok := s.byName[username]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func TestCreateGroupAddsOwnerAndAdmins(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	admin := &authModel.User{Username: "scout"}
	admin.ID = uuid.New()
	groups := &stubGroupRecords{byID: map[uuid.UUID]*model.Group{}}
	svc := NewGroupService(groups, stubUsersByName{byName: map[string]*authModel.User{"scout": admin}}, nil, nil)

	got, err := svc.CreateGroup(context.Background(), owner, request.CreateGroupRequest{
		Name:           "Ha Giang loop",
		AdminUsernames: []string{"scout", "leader", "scout"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ha Giang loop" {
		t.Fatalf("name = %q", got.Name)
	}
	if got.MyRole == nil || *got.MyRole != enum.GroupRoleOwner {
		t.Fatal("creator should be owner")
	}
	if len(groups.members) != 2 {
		t.Fatalf("members = %d, want 2 (owner+admin, skip duplicate/self)", len(groups.members))
	}
	if groups.members[1].Role != enum.GroupRoleAdmin || groups.members[1].UserID != admin.ID {
		t.Fatal("second member should be admin scout")
	}
}

func TestCreateGroupPersistsPolicy(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	groups := &stubGroupRecords{byID: map[uuid.UUID]*model.Group{}}
	svc := NewGroupService(groups, stubUsersByName{}, nil, nil)
	got, err := svc.CreateGroup(context.Background(), owner, request.CreateGroupRequest{
		Name:   "Crew",
		Policy: "  helmets on  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Policy != "helmets on" || groups.created.Policy != "helmets on" {
		t.Fatalf("policy = %q", got.Policy)
	}
}

func TestUpdateGroupPolicyOnly(t *testing.T) {
	id := uuid.New()
	groups := &stubGroupRecords{byID: map[uuid.UUID]*model.Group{id: {Name: "a", Policy: "old"}}}
	svc := NewGroupService(groups, nil, nil, nil)
	policy := "no night riding"
	got, err := svc.UpdateGroup(context.Background(), id, request.UpdateGroupRequest{Policy: &policy}, enum.GroupRoleOwner)
	if err != nil {
		t.Fatal(err)
	}
	if got.Policy != "no night riding" {
		t.Fatalf("policy = %q", got.Policy)
	}
}

func TestCreateGroupUnknownAdminAborts(t *testing.T) {
	owner := &authModel.User{Username: "leader"}
	owner.ID = uuid.New()
	svc := NewGroupService(&stubGroupRecords{}, stubUsersByName{byName: map[string]*authModel.User{}}, nil, nil)
	_, err := svc.CreateGroup(context.Background(), owner, request.CreateGroupRequest{
		Name:           "Trip",
		AdminUsernames: []string{"ghost"},
	})
	if !errors.Is(err, repository.ErrAdminUserNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteGroupBlockedWhenTripsExist(t *testing.T) {
	svc := NewGroupService(&stubGroupRecords{tripCount: 1}, nil, nil, nil)
	err := svc.DeleteGroup(context.Background(), uuid.New())
	if !errors.Is(err, repository.ErrGroupHasTrips) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateGroupRequiresAField(t *testing.T) {
	id := uuid.New()
	svc := NewGroupService(&stubGroupRecords{byID: map[uuid.UUID]*model.Group{id: {Name: "a"}}}, nil, nil, nil)
	_, err := svc.UpdateGroup(context.Background(), id, request.UpdateGroupRequest{}, enum.GroupRoleOwner)
	if !errors.Is(err, repository.ErrNoGroupUpdate) {
		t.Fatalf("got %v", err)
	}
}
