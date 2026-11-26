package service

import (
	"context"
	"errors"
	"fmt"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/utils"

	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid username or password")

type LoginRepository interface {
	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
}

type LoginService struct {
	loginRepository LoginRepository
}

func NewLoginService(repo LoginRepository) *LoginService {
	return &LoginService{loginRepository: repo}
}

func (s *LoginService) Login(ctx context.Context, req request.LoginRequest) (string, string, error) {
	user, err := s.loginRepository.FindUserByUsername(ctx, utils.Santize(req.Username))
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && user == nil) {
		return "", "", ErrInvalidCredentials
	}
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	if valid, err := user.CheckPassword(req.Password); err != nil || !valid {
		return "", "", ErrInvalidCredentials
	}
	return CreateTokenPair(user)
}
