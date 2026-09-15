package service

import (
	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/authentication/model/request"
	"Road-To-Destination-BE/module/authentication/repository"
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func testResetStore(t *testing.T) *repository.PasswordResetStore {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return repository.NewPasswordResetStore(client)
}

func TestForgetPasswordSkipsSecondMailWhileKeyExists(t *testing.T) {
	t.Setenv("SECRET_MAIL_JWT", "test-mail-secret")
	t.Setenv("APP_PUBLIC_URL", "http://localhost:8080")

	user := &model.User{Username: "traveler", Email: "traveler@example.com"}
	user.ID = uuid.New()
	mailer := &stubMailClient{}
	svc := NewForgetPasswordService(stubMailRepo{user: user}, mailer, testResetStore(t))
	req := request.ForgetPasswordRequest{Email: user.Email}

	if _, err := svc.ForgetPassword(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if mailer.kind == "" {
		t.Fatal("expected first mail to send")
	}
	mailer.kind = ""
	got, err := svc.ForgetPassword(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if mailer.kind != "" {
		t.Fatal("expected second request to skip sending mail")
	}
	if got.Message != repository.alreadySentMailMessage {
		t.Fatalf("message = %q", got.Message)
	}
}

func TestForgetPasswordBlockedByResetCooldown(t *testing.T) {
	user := &model.User{Username: "traveler", Email: "traveler@example.com"}
	user.ID = uuid.New()
	store := testResetStore(t)
	if err := store.StartResetCooldown(context.Background(), user.ID); err != nil {
		t.Fatal(err)
	}
	mailer := &stubMailClient{}
	svc := NewForgetPasswordService(stubMailRepo{user: user}, mailer, store)

	_, err := svc.ForgetPassword(context.Background(), request.ForgetPasswordRequest{Email: user.Email})
	var cooldown *repository.PasswordResetCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("got %v, want cooldown", err)
	}
	if mailer.kind != "" {
		t.Fatal("should not send mail during reset cooldown")
	}
}

type stubResetRepo struct {
	calls int
}

func (s *stubResetRepo) ResetPassword(context.Context, uuid.UUID, string) error {
	s.calls++
	return nil
}

func TestResetPasswordClearsForgetKeyAndBlocksReuse(t *testing.T) {
	userID := uuid.New()
	email := "traveler@example.com"
	store := testResetStore(t)
	if err := store.MarkForgetMailSent(context.Background(), userID, email); err != nil {
		t.Fatal(err)
	}
	repo := &stubResetRepo{}
	svc := NewResetPasswordService(repo, store)

	if _, err := svc.ResetPassword(context.Background(), userID, email, "Newpass1!", "Newpass1!"); err != nil {
		t.Fatal(err)
	}
	if repo.calls != 1 {
		t.Fatalf("calls = %d", repo.calls)
	}
	sent, err := store.ForgetMailAlreadySent(context.Background(), userID, email)
	if err != nil {
		t.Fatal(err)
	}
	if sent {
		t.Fatal("forget-pass key should be removed after successful reset")
	}

	_, err = svc.ResetPassword(context.Background(), userID, email, "Otherpass1!", "Otherpass1!")
	var cooldown *repository.PasswordResetCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("got %v, want cooldown", err)
	}
	if repo.calls != 1 {
		t.Fatalf("reuse should not change password again, calls = %d", repo.calls)
	}
	if cooldown.WaitMinutes < repository.cooldownMinutes(2) {
		t.Fatalf("wait = %d, want at least %d", cooldown.WaitMinutes, repository.cooldownMinutes(2))
	}
}

func TestCooldownMinutesCaps(t *testing.T) {
	if got := repository.cooldownMinutes(1); got != 20 {
		t.Fatalf("y=1 got %d", got)
	}
	if got := repository.cooldownMinutes(9); got != 60 {
		t.Fatalf("y=9 got %d", got)
	}
	if got := repository.cooldownMinutes(20); got != 60 {
		t.Fatalf("y=20 got %d", got)
	}
}
