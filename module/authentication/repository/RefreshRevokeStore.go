package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const revokedRefreshKeyPrefix = "auth:revoked-refresh:"

var ErrRevokeStoreUnavailable = errors.New("refresh revoke store is not configured")

type RefreshRevokeStore struct {
	redis   *redis.Client
	persist revokedRefreshPersist
}

func NewRefreshRevokeStore(client *redis.Client, db *gorm.DB) *RefreshRevokeStore {
	store := &RefreshRevokeStore{redis: client}
	if db != nil {
		store.persist = &gormRevokePersist{db: db}
	}
	return store
}

func HashRefreshToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func revokedRefreshKey(hash string) string {
	return revokedRefreshKeyPrefix + hash
}

func (s *RefreshRevokeStore) enabled() bool {
	return s != nil && (s.redis != nil || s.persist != nil)
}

func (s *RefreshRevokeStore) Revoke(ctx context.Context, rawToken, username string, expiresAt time.Time) error {
	if !s.enabled() {
		return ErrRevokeStoreUnavailable
	}
	now := time.Now()
	if !expiresAt.After(now) {
		return nil
	}

	hash := HashRefreshToken(rawToken)
	row := &model.RevokedRefreshToken{
		TokenHash: hash,
		Username:  username,
		ExpiresAt: expiresAt.UTC(),
	}

	if s.persist != nil {
		if err := s.persist.Upsert(ctx, row); err != nil {
			return err
		}
		_ = s.persist.DeleteExpired(ctx, now)
	}

	if err := s.setRedis(ctx, hash, username, time.Until(expiresAt)); err != nil && s.persist == nil {
		return err
	}
	return nil
}

func (s *RefreshRevokeStore) IsRevoked(ctx context.Context, rawToken string) (bool, error) {
	if !s.enabled() {
		return false, nil
	}

	hash := HashRefreshToken(rawToken)
	now := time.Now()
	redisTrustedMiss := false

	if s.redis != nil {
		n, err := s.redis.Exists(ctx, revokedRefreshKey(hash)).Result()
		if err == nil {
			if n > 0 {
				return true, nil
			}
			redisTrustedMiss = true
		} else if s.persist == nil {
			return false, fmt.Errorf("check revoked refresh: %w", err)
		}
	}

	if s.persist != nil {
		row, err := s.persist.FindActive(ctx, hash, now)
		if err != nil {
			return false, err
		}
		if row == nil {
			return false, nil
		}
		_ = s.setRedis(ctx, hash, row.Username, time.Until(row.ExpiresAt))
		return true, nil
	}

	if redisTrustedMiss {
		return false, nil
	}
	return false, nil
}

func (s *RefreshRevokeStore) setRedis(ctx context.Context, hash, username string, ttl time.Duration) error {
	if s == nil || s.redis == nil {
		return nil
	}
	if ttl <= 0 {
		return nil
	}
	if username == "" {
		username = "1"
	}
	return s.redis.Set(ctx, revokedRefreshKey(hash), username, ttl).Err()
}
