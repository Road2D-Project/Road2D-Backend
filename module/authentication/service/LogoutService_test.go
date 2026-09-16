package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type stubRevokeStore struct {
	revoked map[string]time.Time
	err     error
}

func (s *stubRevokeStore) Revoke(_ context.Context, rawToken, _ string, expiresAt time.Time) error {
	if s.err != nil {
		return s.err
	}
	if s.revoked == nil {
		s.revoked = make(map[string]time.Time)
	}
	s.revoked[rawToken] = expiresAt
	return nil
}

func (s *stubRevokeStore) IsRevoked(_ context.Context, rawToken string) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	_, ok := s.revoked[rawToken]
	return ok, nil
}

func TestLogoutRevokesValidRefresh(t *testing.T) {
	t.Setenv("SECRET_REFRESH_JWT", "test-refresh-secret")
	token, err := CreateRefreshToken("traveler")
	if err != nil {
		t.Fatal(err)
	}
	store := &stubRevokeStore{}
	svc := NewLogoutService(store)
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.revoked[token]; !ok {
		t.Fatal("expected token to be revoked")
	}
}

func TestLogoutExpiredRefreshIsNoop(t *testing.T) {
	t.Setenv("SECRET_REFRESH_JWT", "test-refresh-secret")
	token := mustRefreshJWT(t, "traveler", time.Now().Add(-time.Hour))
	store := &stubRevokeStore{}
	svc := NewLogoutService(store)
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if len(store.revoked) != 0 {
		t.Fatal("expired token should not be persisted")
	}
}

func TestAssertRefreshUsableRejectsRevoked(t *testing.T) {
	t.Setenv("SECRET_REFRESH_JWT", "test-refresh-secret")
	t.Setenv("SECRET_JWT", "test-access-secret")
	token, err := CreateRefreshToken("traveler")
	if err != nil {
		t.Fatal(err)
	}
	store := &stubRevokeStore{}
	svc := NewLogoutService(store)
	if err := svc.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	_, err = svc.AssertRefreshUsable(context.Background(), token)
	if !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("got %v, want ErrRefreshRevoked", err)
	}
}

func TestAssertRefreshUsableAllowsFreshToken(t *testing.T) {
	t.Setenv("SECRET_REFRESH_JWT", "test-refresh-secret")
	token, err := CreateRefreshToken("traveler")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewLogoutService(&stubRevokeStore{})
	claims, err := svc.AssertRefreshUsable(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if claims["username"] != "traveler" {
		t.Fatalf("username = %v", claims["username"])
	}
}

func mustRefreshJWT(t *testing.T, username string, exp time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"authorized": true,
		"username":   username,
		"exp":        exp.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte("test-refresh-secret"))
	if err != nil {
		t.Fatal(err)
	}
	return signed
}
