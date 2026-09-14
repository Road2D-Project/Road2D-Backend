package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type ResetPasswordRepository interface {
	ResetPassword(ctx context.Context, userId uuid.UUID, newPassword string) error
}

type ResetPasswordService struct {
	repo  ResetPasswordRepository
	store *PasswordResetStore
}

func NewResetPasswordService(repo ResetPasswordRepository, store *PasswordResetStore) *ResetPasswordService {
	return &ResetPasswordService{repo: repo, store: store}
}

func (u *ResetPasswordService) ResetPassword(ctx context.Context, userId uuid.UUID, email, password, confirmPassword string) (string, error) {
	if password != confirmPassword {
		return "", errors.New("Password and confirm password do not match")
	}
	_, blocked, err := u.store.ResetCooldownRemaining(ctx, userId)
	if err != nil {
		return "", err
	}
	if blocked {
		wait, punishErr := u.store.PunishResetSpam(ctx, userId)
		if punishErr != nil {
			return "", punishErr
		}
		return "", &PasswordResetCooldownError{WaitMinutes: wait}
	}
	if err := u.repo.ResetPassword(ctx, userId, password); err != nil {
		return "", err
	}
	if err := u.store.ClearForgetMail(ctx, userId, email); err != nil {
		return "", err
	}
	if err := u.store.StartResetCooldown(ctx, userId); err != nil {
		return "", err
	}
	return passwordResetSuccessMessage, nil
}
