package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/mail"
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type stubMailRepo struct {
	user *model.User
	err  error
}

func (s stubMailRepo) CheckExistedEmail(context.Context, string) (*model.User, error) {
	return s.user, s.err
}

type stubMailClient struct {
	kind mail.Kind
	to   string
	form any
}

func (s *stubMailClient) Send(_ context.Context, kind mail.Kind, to string, form any) error {
	s.kind = kind
	s.to = to
	s.form = form
	return nil
}

func TestForgetPasswordSendsTypedMail(t *testing.T) {
	t.Setenv("SECRET_MAIL_JWT", "test-mail-secret")
	t.Setenv("APP_PUBLIC_URL", "http://localhost:8080")

	user := &model.User{Username: "traveler", Email: "traveler@example.com"}
	user.ID = uuid.New()
	mailer := &stubMailClient{}
	svc := NewForgetPasswordService(stubMailRepo{user: user}, mailer, nil)

	got, err := svc.ForgetPassword(context.Background(), request.ForgetPasswordRequest{Email: "traveler@example.com"})
	if err != nil {
		t.Fatalf("ForgetPassword: %v", err)
	}
	if got == nil || got.Message == "" {
		t.Fatal("expected a confirmation message")
	}
	if mailer.kind != mail.KindForgetPassword {
		t.Fatalf("kind = %q, want %q", mailer.kind, mail.KindForgetPassword)
	}
	if mailer.to != user.Email {
		t.Fatalf("to = %q, want %q", mailer.to, user.Email)
	}
	form, ok := mailer.form.(mail.ForgetPasswordForm)
	if !ok {
		t.Fatalf("form type %T", mailer.form)
	}
	if form.Username != user.Username {
		t.Fatalf("username = %q", form.Username)
	}
	if !strings.Contains(form.ResetURL, "http://localhost:8080/v1/auth/user/reset-password/") {
		t.Fatalf("reset url missing token path: %s", form.ResetURL)
	}
}
