package repository

import (
	"Road-To-Destination-BE/utils/enum"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const groupMemberRoleCacheTTL = 15 * time.Minute

type GroupMemberStore struct {
	redisClient *redis.Client
}

func NewGroupMemberStore(redisClient *redis.Client) *GroupMemberStore {
	if redisClient == nil {
		return nil
	}
	return &GroupMemberStore{redisClient: redisClient}
}

func (g *GroupMemberStore) enabled() bool {
	return g != nil && g.redisClient != nil
}

func (g *GroupMemberStore) GetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID) (enum.GroupRole, error) {
	if !g.enabled() {
		return 0, ErrMemberRoleNotInCache
	}
	roleVal, err := g.redisClient.Get(ctx, g.GroupMemberRoleCacheKey(groupId, userId)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrMemberRoleNotInCache
	}
	if err != nil {
		return 0, err
	}
	role, err := enum.GroupRoleString(roleVal)
	if err != nil {
		return 0, ErrMemberRoleNotInCache
	}
	return role, nil
}

func (g *GroupMemberStore) SetCachedGroupMemberRole(ctx context.Context, groupId uuid.UUID, userId uuid.UUID, role enum.GroupRole) error {
	if !g.enabled() {
		return nil
	}
	return g.redisClient.Set(ctx, g.GroupMemberRoleCacheKey(groupId, userId), role.String(), groupMemberRoleCacheTTL).Err()
}

func (g *GroupMemberStore) GroupMemberRoleCacheKey(groupId uuid.UUID, userId uuid.UUID) string {
	return fmt.Sprintf("group-role:%s:%s", groupId.String(), userId.String())
}
