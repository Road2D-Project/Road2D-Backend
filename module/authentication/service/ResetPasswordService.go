package service

import (
	"Road-To-Destination-BE/module/authentication/repository"
	"context"
	"errors"
	"log"

	"github.com/google/uuid"
)

type ResetPasswordRepository interface {
	ResetPassword(ctx context.Context, userId uuid.UUID, newPassword string, expectedVersion int64) error
}

type ResetPasswordService struct {
	repo  ResetPasswordRepository
	store *repository.PasswordResetStore
}

func NewResetPasswordService(repo ResetPasswordRepository, store *repository.PasswordResetStore) *ResetPasswordService {
	return &ResetPasswordService{repo: repo, store: store}
}

func (u *ResetPasswordService) ResetPassword(ctx context.Context, userId uuid.UUID, email, password, confirmPassword string, expectedVersion int64) (string, error) {
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
		return "", &repository.PasswordResetCooldownError{WaitMinutes: wait}
	}
	if err := u.repo.ResetPassword(ctx, userId, password, expectedVersion); err != nil {
		return "", err
	}
	if err := u.store.ClearForgetMail(ctx, userId, email); err != nil {
		log.Printf("auth: password reset mail marker cleanup failed: %v", err)
	}
	if err := u.store.StartResetCooldown(ctx, userId); err != nil {
		log.Printf("auth: password reset cooldown update failed: %v", err)
	}
	return repository.PasswordResetSuccessMessage, nil
}
