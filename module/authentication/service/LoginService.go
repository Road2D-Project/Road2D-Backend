package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/utils"
	"context"
	"errors"
)

type LoginRepository interface {
	FindUserByUsername(ctx context.Context, username string) (*model.User, error)
}

type LoginService struct {
	loginRepository LoginRepository
}

func NewLoginService(loginRepository LoginRepository) *LoginService {
	return &LoginService{loginRepository: loginRepository}
}
func (service *LoginService) Login(ctx context.Context, request request.LoginRequest) (string, string, error) {
	user, err := service.loginRepository.FindUserByUsername(ctx, utils.Santize(request.Username))
	if err != nil {
		return "", "", errors.New("user not found")
	}
	if ok, errCheck := user.CheckPassword(request.Password); errCheck == nil && ok == true {
		accessToken, err := CreateToken(request.Username)
		refreshToken, err := CreateRefreshToken(request.Username)
		if err == nil {
			if err != nil {
				return "", "", err
			}
			return accessToken, refreshToken, err
		}

	} else if errCheck != nil {
		return "", "", errCheck
	} else if ok == false {
		return "", "", errors.New("invalid password")
	}
	return "", "", err
}
