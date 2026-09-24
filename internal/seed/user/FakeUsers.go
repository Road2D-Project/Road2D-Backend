package user

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"Road-To-Destination-BE/module/authentication/model/request"

	"gorm.io/gorm"
)

//go:embed users.json
var defaultUsersJSON []byte

type FakeUser struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type FakeUserReport struct {
	Created []string
	Skipped []string
}

func DefaultFakeUsers() ([]FakeUser, error) {
	return decodeFakeUsers(defaultUsersJSON)
}

func LoadFakeUsers(path string) ([]FakeUser, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeFakeUsers(raw)
}

func SeedFakeUsers(ctx context.Context, register RegisterUserService, repository UserRepository, users []FakeUser) (*FakeUserReport, error) {
	report := &FakeUserReport{}
	for _, account := range users {
		_, err := repository.FindUserByUsername(ctx, account.Username)
		if err == nil {
			report.Skipped = append(report.Skipped, account.Username)
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return report, err
		}
		_, err = register.Register(request.RegisterRequest{
			Username:        account.Username,
			Email:           account.Email,
			Password:        account.Password,
			ConfirmPassword: account.Password,
		}, ctx)
		if err != nil {
			return report, fmt.Errorf("register %s: %w", account.Username, err)
		}
		report.Created = append(report.Created, account.Username)
	}
	return report, nil
}

func decodeFakeUsers(raw []byte) ([]FakeUser, error) {
	var users []FakeUser
	if err := json.Unmarshal(raw, &users); err != nil {
		return nil, fmt.Errorf("fake users json: %w", err)
	}
	if len(users) == 0 {
		return nil, errors.New("fake users json is empty")
	}
	seenName := map[string]struct{}{}
	seenMail := map[string]struct{}{}
	for i, account := range users {
		if account.Username == "" || account.Email == "" || account.Password == "" {
			return nil, fmt.Errorf("fake user %d is missing username, email, or password", i)
		}
		if _, ok := seenName[account.Username]; ok {
			return nil, fmt.Errorf("duplicate username %q", account.Username)
		}
		if _, ok := seenMail[account.Email]; ok {
			return nil, fmt.Errorf("duplicate email %q", account.Email)
		}
		seenName[account.Username] = struct{}{}
		seenMail[account.Email] = struct{}{}
	}
	return users, nil
}
