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
	repo ResetPasswordRepository
}

func NewResetPasswordService(repo ResetPasswordRepository) *ResetPasswordService {
	return &ResetPasswordService{repo: repo}
}

func (u *ResetPasswordService) ResetPassword(ctx context.Context, userId uuid.UUID, password, confirmPassword string) (string, error) {
	if password != confirmPassword {
		return "", errors.New("Password and confirm password do not match")
	}
	err := u.repo.ResetPassword(ctx, userId, password)
	if err != nil {
		return "", err
	}
	return "Reset Password", nil
}
