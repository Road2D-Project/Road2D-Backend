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

const tripMemberRoleCacheTTL = 15 * time.Minute

type TripMemberStore struct {
	redisClient *redis.Client
}

func NewTripMemberStore(redisClient *redis.Client) *TripMemberStore {
	if redisClient == nil {
		return nil
	}
	return &TripMemberStore{redisClient: redisClient}
}

func (s *TripMemberStore) enabled() bool {
	return s != nil && s.redisClient != nil
}

func (s *TripMemberStore) GetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) (enum.TripRole, error) {
	if !s.enabled() {
		return 0, ErrMemberRoleNotInCache
	}
	roleVal, err := s.redisClient.Get(ctx, s.TripMemberRoleCacheKey(tripId, userId)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrMemberRoleNotInCache
	}
	if err != nil {
		return 0, err
	}
	role, err := enum.TripRoleString(roleVal)
	if err != nil {
		return 0, ErrMemberRoleNotInCache
	}
	return role, nil
}

func (s *TripMemberStore) SetCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID, role enum.TripRole) error {
	if !s.enabled() {
		return nil
	}
	return s.redisClient.Set(ctx, s.TripMemberRoleCacheKey(tripId, userId), role.String(), tripMemberRoleCacheTTL).Err()
}

func (s *TripMemberStore) DeleteCachedTripMemberRole(ctx context.Context, tripId uuid.UUID, userId uuid.UUID) error {
	if !s.enabled() {
		return nil
	}
	return s.redisClient.Del(ctx, s.TripMemberRoleCacheKey(tripId, userId)).Err()
}

func (s *TripMemberStore) TripMemberRoleCacheKey(tripId uuid.UUID, userId uuid.UUID) string {
	return fmt.Sprintf("trip-role:%s:%s", tripId.String(), userId.String())
}
