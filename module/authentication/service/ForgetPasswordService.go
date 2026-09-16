package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/model/response"
	"Road-To-Destination-BE/module/authentication/repository"
	"Road-To-Destination-BE/module/mail"
	"Road-To-Destination-BE/module/share"
	"Road-To-Destination-BE/utils"
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type MailRepository interface {
	CheckExistedEmail(ctx context.Context, mail string) (*model.User, error)
}

type MailClient interface {
	Send(ctx context.Context, kind mail.Kind, to string, form any) error
}

type ForgetPasswordService struct {
	emailRepository MailRepository
	mailClient      MailClient
	store           *repository.PasswordResetStore
}

func NewForgetPasswordService(emailRepository MailRepository, mailClient MailClient, store *repository.PasswordResetStore) *ForgetPasswordService {
	return &ForgetPasswordService{emailRepository: emailRepository, mailClient: mailClient, store: store}
}

func (u *ForgetPasswordService) ForgetPassword(ctx context.Context, request request.ForgetPasswordRequest) (*response.ForgetPasswordResponse, error) {
	user, err := u.emailRepository.CheckExistedEmail(ctx, utils.Santize(request.Email))
	if err != nil {
		return nil, err
	}
	wait, blocked, err := u.store.ResetCooldownRemaining(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, &repository.PasswordResetCooldownError{WaitMinutes: wait}
	}
	alreadySent, err := u.store.ForgetMailAlreadySent(ctx, user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	if alreadySent {
		return &response.ForgetPasswordResponse{Message: repository.AlreadySentMailMessage}, nil
	}
	if err := u.SendMail(ctx, ForgetPasswordForm{
		Email:    user.Email,
		Username: user.Username,
		UserId:   user.ID,
	}); err != nil {
		return nil, err
	}
	if err := u.store.MarkForgetMailSent(ctx, user.ID, user.Email); err != nil {
		return nil, err
	}
	return &response.ForgetPasswordResponse{Message: repository.AlreadySentMailMessage}, nil
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
		ExpireMinutes: repository.ResetTokenTTLMinutes,
	})
}

type ForgetPasswordForm struct {
	Username string
	UserId   uuid.UUID
	Email    string
}
