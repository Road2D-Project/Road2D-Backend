package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/mail"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const resetTokenTTLMinutes = 15

type MailRepository interface {
	CheckExistedEmail(ctx context.Context, mail string) (*model.User, error)
}

type MailClient interface {
	Send(ctx context.Context, kind mail.Kind, to string, form any) error
}

type ForgetPasswordService struct {
	emailRepository MailRepository
	mailClient      MailClient
}

func NewForgetPasswordService(emailRepository MailRepository, mailClient MailClient) *ForgetPasswordService {
	return &ForgetPasswordService{emailRepository: emailRepository, mailClient: mailClient}
}

func (u *ForgetPasswordService) ForgetPassword(ctx context.Context, request request.ForgetPasswordRequest) (*response.ForgetPasswordResponse, error) {
	user, err := u.emailRepository.CheckExistedEmail(ctx, utils.Santize(request.Email))
	if err != nil {
		return nil, err
	}
	sendEmailErr := u.SendMail(ctx, ForgetPasswordForm{
		Email:    user.Email,
		Username: user.Username,
		UserId:   user.ID,
	})
	if sendEmailErr != nil {
		return nil, sendEmailErr
	}
	return &response.ForgetPasswordResponse{Message: "Reset password email has been sent"}, nil
}

func (u *ForgetPasswordService) SendMail(ctx context.Context, form ForgetPasswordForm) error {
	if u.mailClient == nil {
		return fmt.Errorf("mail sender is not configured")
	}
	token, err := CreateResetPasswordToken(form)
	if err != nil {
		return err
	}
	baseURL := strings.TrimRight(share.GetEnvStringDefault("APP_PUBLIC_URL", "http://localhost:8080"), "/")
	resetURL := fmt.Sprintf("%s/v1/auth/user/reset-password/%s", baseURL, token)
	return u.mailClient.Send(ctx, mail.KindForgetPassword, form.Email, mail.ForgetPasswordForm{
		Username:      form.Username,
		ResetURL:      resetURL,
		ExpireMinutes: resetTokenTTLMinutes,
	})
}

type ForgetPasswordForm struct {
	Username string
	UserId   uuid.UUID
	Email    string
}
