package user

import (
	"context"
	"errors"
	"os"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func DefaultUser() (*model.User, error) {
	username := os.Getenv("USER_NAME")
	email := os.Getenv("USER_EMAIL")
	password := os.Getenv("USER_PASSWORD")
	if username == "" || email == "" || password == "" {
		return nil, errors.New("set USER_NAME, USER_EMAIL, and USER_PASSWORD")
	}
	return &model.User{
		Username: username,
		Email:    email,
		Password: password,
	}, nil
}

type LoginUserService interface {
	Login(ctx context.Context, request request.LoginRequest) (string, string, error)
}

type RegisterUserService interface {
	Register(request request.RegisterRequest, ctx context.Context) (uuid.UUID, error)
}

type UserRepository interface {
	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
}

func LoginDefaultUser(ctx context.Context, userService LoginUserService, registerService RegisterUserService, repository UserRepository) (string, string, error) {
	creds, err := DefaultUser()
	if err != nil {
		return "", "", err
	}
	_, err = repository.FindUserByUsername(ctx, creds.Username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		_, err = registerService.Register(request.RegisterRequest{
			Username:        creds.Username,
			Email:           creds.Email,
			Password:        creds.Password,
			ConfirmPassword: creds.Password,
		}, ctx)
	}
	if err != nil {
		return "", "", err
	}
	return userService.Login(ctx, request.LoginRequest{
		Username: creds.Username,
		Password: creds.Password,
	})
}
