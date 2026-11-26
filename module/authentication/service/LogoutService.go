package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type refreshRevokeStore interface {
	Revoke(ctx context.Context, rawToken, username string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, rawToken string) (bool, error)
}

type LogoutService struct {
	store refreshRevokeStore
	users LoginRepository
}

func NewLogoutService(store refreshRevokeStore, users LoginRepository) *LogoutService {
	return &LogoutService{store: store, users: users}
}

func (s *LogoutService) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return ErrTokenInvalid
	}
	claims, err := VerifyRefreshToken(refreshToken)
	if err != nil {
		if errors.Is(err, ErrTokenExpired) {
			return nil
		}
		return err
	}
	if s == nil || s.store == nil {
		return repository.ErrRevokeStoreUnavailable
	}
	exp, ok := ClaimsExpiry(claims)
	if !ok {
		return ErrTokenInvalid
	}
	username, _ := claims["username"].(string)
	if sessions, ok := s.store.(interface {
		RevokeSession(context.Context, string, string, uuid.UUID, string, int64, time.Time) error
	}); ok {
		id, _ := uuid.Parse(claims["sub"].(string))
		sid, _ := claims["sid"].(string)
		version, _ := ClaimsSessionVersion(claims)
		return sessions.RevokeSession(ctx, refreshToken, username, id, sid, version, exp)
	}
	return s.store.Revoke(ctx, refreshToken, username, exp)
}

func (s *LogoutService) AssertRefreshUsable(ctx context.Context, refreshToken string) (*model.User, error) {
	claims, err := VerifyRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil || s.users == nil {
		return nil, ErrAuthUnavailable
	}
	revoked, err := s.store.IsRevoked(ctx, refreshToken)
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	if revoked {
		return nil, ErrRefreshRevoked
	}
	username, _ := claims["username"].(string)
	user, err := s.users.FindUserByUsername(ctx, username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSessionRevoked
	}
	if err != nil {
		return nil, ErrAuthUnavailable
	}
	if err := ValidateUserSession(claims, user); err != nil {
		return nil, err
	}
	return user, nil
}
