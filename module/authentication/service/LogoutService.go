package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"Road-To-Destination-BE/module/authentication/repository"

	"github.com/dgrijalva/jwt-go"
)

type refreshRevokeStore interface {
	Revoke(ctx context.Context, rawToken, username string, expiresAt time.Time) error
	IsRevoked(ctx context.Context, rawToken string) (bool, error)
}

type LogoutService struct {
	store refreshRevokeStore
}

func NewLogoutService(store refreshRevokeStore) *LogoutService {
	return &LogoutService{store: store}
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
	return s.store.Revoke(ctx, refreshToken, username, exp)
}

func (s *LogoutService) AssertRefreshUsable(ctx context.Context, refreshToken string) (jwt.MapClaims, error) {
	claims, err := VerifyRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if s == nil || s.store == nil {
		return claims, nil
	}
	revoked, err := s.store.IsRevoked(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	if revoked {
		return nil, ErrRefreshRevoked
	}
	return claims, nil
}
