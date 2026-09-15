package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	resetTokenTTLMinutes        = 15
	resetCooldownBaseMinutes    = 15
	resetCooldownStepMinutes    = 5
	resetCooldownMaxMinutes     = 60
	alreadySentMailMessage      = "Reset password email has been sent"
	PasswordResetSuccessMessage = "Reset Password"
)

type PasswordResetCooldownError struct {
	WaitMinutes int
}

func (e *PasswordResetCooldownError) Error() string {
	return fmt.Sprintf("Bạn vừa mới đổi mật khẩu, hãy thử lại sau: %d phút", e.WaitMinutes)
}

func forgetPassKey(userID uuid.UUID, email string) string {
	return fmt.Sprintf("forget-pass:%s:%s", userID.String(), email)
}

func resetPassKey(userID uuid.UUID) string {
	return fmt.Sprintf("reset-pass:%s", userID.String())
}

func cooldownMinutes(attempt int) int {
	if attempt < 1 {
		attempt = 1
	}
	x := resetCooldownBaseMinutes + attempt*resetCooldownStepMinutes
	if x > resetCooldownMaxMinutes {
		return resetCooldownMaxMinutes
	}
	return x
}

func remainingMinutes(ttl time.Duration) int {
	if ttl <= 0 {
		return 1
	}
	minutes := int(ttl / time.Minute)
	if ttl%time.Minute != 0 {
		minutes++
	}
	if minutes < 1 {
		return 1
	}
	return minutes
}

type PasswordResetStore struct {
	redis *redis.Client
}

func NewPasswordResetStore(client *redis.Client) *PasswordResetStore {
	if client == nil {
		return nil
	}
	return &PasswordResetStore{redis: client}
}

func (s *PasswordResetStore) enabled() bool {
	return s != nil && s.redis != nil
}

func (s *PasswordResetStore) ForgetMailAlreadySent(ctx context.Context, userID uuid.UUID, email string) (bool, error) {
	if !s.enabled() {
		return false, nil
	}
	n, err := s.redis.Exists(ctx, forgetPassKey(userID, email)).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *PasswordResetStore) MarkForgetMailSent(ctx context.Context, userID uuid.UUID, email string) error {
	if !s.enabled() {
		return nil
	}
	return s.redis.Set(ctx, forgetPassKey(userID, email), 1, time.Duration(resetTokenTTLMinutes)*time.Minute).Err()
}

func (s *PasswordResetStore) ClearForgetMail(ctx context.Context, userID uuid.UUID, email string) error {
	if !s.enabled() {
		return nil
	}
	return s.redis.Del(ctx, forgetPassKey(userID, email)).Err()
}

func (s *PasswordResetStore) ResetCooldownRemaining(ctx context.Context, userID uuid.UUID) (int, bool, error) {
	if !s.enabled() {
		return 0, false, nil
	}
	ttl, err := s.redis.TTL(ctx, resetPassKey(userID)).Result()
	if err != nil {
		return 0, false, err
	}
	if ttl <= 0 {
		return 0, false, nil
	}
	return remainingMinutes(ttl), true, nil
}

func (s *PasswordResetStore) PunishResetSpam(ctx context.Context, userID uuid.UUID) (int, error) {
	if !s.enabled() {
		return 0, nil
	}
	key := resetPassKey(userID)
	exists, err := s.redis.Exists(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if exists == 0 {
		return 0, nil
	}
	y, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	minutes := cooldownMinutes(int(y))
	if err := s.redis.Expire(ctx, key, time.Duration(minutes)*time.Minute).Err(); err != nil {
		return 0, err
	}
	return minutes, nil
}

func (s *PasswordResetStore) StartResetCooldown(ctx context.Context, userID uuid.UUID) error {
	if !s.enabled() {
		return nil
	}
	minutes := cooldownMinutes(1)
	return s.redis.Set(ctx, resetPassKey(userID), 1, time.Duration(minutes)*time.Minute).Err()
}
