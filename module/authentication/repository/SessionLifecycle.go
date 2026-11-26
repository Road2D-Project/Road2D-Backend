package repository

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/share/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserAccessLifecycle interface {
	UserChange(context.Context, uuid.UUID, string, int64, func(context.Context) error) error
}

func (u *UserRepository) WithAccessLifecycle(lifecycle UserAccessLifecycle) *UserRepository {
	u.lifecycle = lifecycle
	return u
}

func (u *UserRepository) IsLoginSessionRevoked(ctx context.Context, sid string) (bool, error) {
	if u.db == nil {
		return false, ErrRevokeStoreUnavailable
	}
	var count int64
	err := u.db.WithContext(ctx).Model(&model.RevokedRefreshToken{}).
		Where("session_id = ? AND expires_at > clock_timestamp()", sid).Count(&count).Error
	return count > 0, err
}

func (s *RefreshRevokeStore) WithAccessLifecycle(lifecycle UserAccessLifecycle) *RefreshRevokeStore {
	s.lifecycle = lifecycle
	return s
}

func (s *RefreshRevokeStore) RevokeSession(ctx context.Context, raw, username string, userID uuid.UUID, sid string, version int64, expiry time.Time) error {
	if s.db == nil {
		return s.Revoke(ctx, raw, username, expiry)
	}
	before := int64(0)
	var sessionID *uuid.UUID
	if sid != "" {
		parsed, err := uuid.Parse(sid)
		if err != nil {
			return err
		}
		sessionID = &parsed
	} else {
		// Older tokens cannot be paired with one login. Revoke that account
		// version once, instead of leaving legacy access JWTs usable.
		before = version + 1
	}
	change := func(ctx context.Context) error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			row := model.RevokedRefreshToken{TokenHash: HashRefreshToken(raw), Username: username, SessionID: sessionID, ExpiresAt: expiry.UTC()}
			created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if created.Error != nil || created.RowsAffected == 0 {
				return created.Error
			}
			if before > 0 {
				if err := tx.Model(&model.User{}).Where("id = ? AND session_version = ?", userID, version).
					Update("session_version", gorm.Expr("session_version + 1")).Error; err != nil {
					return err
				}
			}
			return outbox.AppendSessionRevoked(tx, userID, sid, before)
		})
	}
	var err error
	if s.lifecycle != nil {
		err = s.lifecycle.UserChange(ctx, userID, sid, before, change)
	} else {
		err = change(ctx)
	}
	if err == nil {
		_ = s.persist.DeleteExpired(ctx, time.Now())
		_ = s.setRedis(ctx, HashRefreshToken(raw), username, time.Until(expiry))
	}
	return err
}
