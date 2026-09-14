package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/utils"
	"context"
	"errors"

	"github.com/google/uuid"
)

type UserRegisterRepository interface {
	Register(username string, password string, email string, ctx context.Context) (uuid.UUID, error)
}

type RegisterService struct {
	repo UserRegisterRepository
}

func NewRegisterService(repo UserRegisterRepository) *RegisterService {
	return &RegisterService{repo: repo}
}

func (s *RegisterService) Register(request request.RegisterRequest, ctx context.Context) (uuid.UUID, error) {
	if request.Password != request.ConfirmPassword {
		return uuid.Nil, errors.New("Passwords do not match")
	}
	username := utils.Santize(request.Username)
	password, err := model.HashPassword(request.Password)
	if err != nil {
		return uuid.Nil, err
	}
	email := utils.Santize(request.Email)
	userId, errRepo := s.repo.Register(username, password, email, ctx)
	if errRepo != nil {
		return uuid.Nil, errRepo
	}
	return userId, nil
}
