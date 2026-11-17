package service

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/authentication/model/request"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func TestLoginUsesCurrentCredentialAndSessionVersion(t *testing.T) {
	setTestJWTSecrets(t)
	user := testSessionUser()
	user.SessionVersion = 2
	hash, err := bcrypt.GenerateFromPassword([]byte("NewPassword1!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user.Password = string(hash)
	svc := NewLoginService(&stubSessionUsers{user: user})
	if _, _, err := svc.Login(context.Background(), request.LoginRequest{Username: user.Username, Password: "OldPassword1!"}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("old password returned %v", err)
	}
	access, refresh, err := svc.Login(context.Background(), request.LoginRequest{Username: " traveler ", Password: "NewPassword1!"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyAccessToken(access)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUserSession(claims, user); err != nil {
		t.Fatal(err)
	}
	claims, err = VerifyRefreshToken(refresh)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateUserSession(claims, user); err != nil {
		t.Fatal(err)
	}
}

func TestLoginReturnsSameCredentialErrorForUnknownUserAndWrongPassword(t *testing.T) {
	user := testSessionUser()
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectPassword1!"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user.Password = string(hash)
	for _, repo := range []*stubSessionUsers{{err: gorm.ErrRecordNotFound}, {user: user}} {
		_, _, err := NewLoginService(repo).Login(context.Background(), request.LoginRequest{Username: "traveler", Password: "WrongPassword1!"})
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("got %v, want uniform credential error", err)
		}
	}
}
