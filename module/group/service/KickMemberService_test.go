package service

import (
	"Road-To-Destination-BE/module/group/model"
	"Road-To-Destination-BE/module/group/repository"
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type stubKickMembers struct {
	member  *model.GroupMember
	kicked  *model.GroupMember
	findErr error
	kickErr error
}

func (s *stubKickMembers) FindMemberById(context.Context, uuid.UUID, uuid.UUID) (*model.GroupMember, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	if s.member == nil {
		return nil, repository.ErrUserNotGroupMember
	}
	copied := *s.member
	return &copied, nil
}

func (s *stubKickMembers) ApplyKick(_ context.Context, member *model.GroupMember) error {
	if s.kickErr != nil {
		return s.kickErr
	}
	copied := *member
	s.kicked = &copied
	return nil
}

type stubKickCache struct {
	deletedUser uuid.UUID
}

func (s *stubKickCache) DeleteCachedGroupMemberRole(_ context.Context, _, userId uuid.UUID) error {
	s.deletedUser = userId
	return nil
}

func TestKickOwnerRemovesAdmin(t *testing.T) {
	actorID, targetID := uuid.New(), uuid.New()
	repo := &stubKickMembers{member: &model.GroupMember{
		UserID: targetID, Role: enum.GroupRoleAdmin, Status: enum.MembershipActive,
	}}
	cache := &stubKickCache{}
	svc := NewKickMemberService(repo, cache)
	if err := svc.Kick(context.Background(), actorID, targetID, uuid.New(), enum.GroupRoleOwner); err != nil {
		t.Fatal(err)
	}
	if repo.kicked == nil || cache.deletedUser != targetID {
		t.Fatal("kick should persist and drop cached role")
	}
}

func TestKickAdminRemovesMember(t *testing.T) {
	svc := NewKickMemberService(&stubKickMembers{member: &model.GroupMember{
		UserID: uuid.New(), Role: enum.GroupRoleMember, Status: enum.MembershipActive,
	}}, nil)
	if err := svc.Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.GroupRoleAdmin); err != nil {
		t.Fatal(err)
	}
}

func TestKickAdminCannotKickAdmin(t *testing.T) {
	svc := NewKickMemberService(&stubKickMembers{member: &model.GroupMember{
		UserID: uuid.New(), Role: enum.GroupRoleAdmin, Status: enum.MembershipActive,
	}}, nil)
	err := svc.Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.GroupRoleAdmin)
	if !errors.Is(err, repository.ErrInsufficientKickRole) {
		t.Fatalf("got %v", err)
	}
}

func TestKickOwnerForbidden(t *testing.T) {
	svc := NewKickMemberService(&stubKickMembers{member: &model.GroupMember{
		UserID: uuid.New(), Role: enum.GroupRoleOwner, Status: enum.MembershipActive,
	}}, nil)
	err := svc.Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.GroupRoleAdmin)
	if !errors.Is(err, repository.ErrCannotKickOwner) {
		t.Fatalf("got %v", err)
	}
}

func TestKickSelfForbidden(t *testing.T) {
	id := uuid.New()
	svc := NewKickMemberService(&stubKickMembers{}, nil)
	err := svc.Kick(context.Background(), id, id, uuid.New(), enum.GroupRoleOwner)
	if !errors.Is(err, repository.ErrCannotKickSelf) {
		t.Fatalf("got %v", err)
	}
}

func TestKickInactiveForbidden(t *testing.T) {
	svc := NewKickMemberService(&stubKickMembers{member: &model.GroupMember{
		UserID: uuid.New(), Role: enum.GroupRoleMember, Status: enum.MembershipInvited,
	}}, nil)
	err := svc.Kick(context.Background(), uuid.New(), uuid.New(), uuid.New(), enum.GroupRoleOwner)
	if !errors.Is(err, repository.ErrUserNotGroupMember) {
		t.Fatalf("got %v", err)
	}
}
