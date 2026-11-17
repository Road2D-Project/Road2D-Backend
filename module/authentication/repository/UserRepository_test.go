package repository

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestResetPasswordRejectsReuseAndPropagatesWriteErrors(t *testing.T) {
	const oldPassword = "OldPassword1!"
	oldHash, err := bcrypt.GenerateFromPassword([]byte(oldPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	writeError := errors.New("password update failed")
	for _, test := range []struct {
		name            string
		password        string
		writeError      error
		wantError       bool
		wantUpdates     int
		expectedVersion int64
		affected        int64
	}{
		{"different password is saved", "NewPassword1!", nil, false, 1, 1, 1},
		{"same password is rejected", oldPassword, nil, true, 0, 1, 1},
		{"database error is returned", "NewPassword1!", writeError, true, 1, 1, 1},
		{"stale reset token is rejected", "NewPassword1!", nil, true, 0, 2, 1},
		{"concurrent reset is rejected", "NewPassword1!", nil, true, 1, 1, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(postgres.New(postgres.Config{
				DSN: "host=127.0.0.1 port=5432 user=test dbname=test sslmode=disable",
			}), &gorm.Config{DisableAutomaticPing: true, SkipDefaultTransaction: true})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			userID := uuid.New()
			// Replace database operations so this regression test needs no server.
			if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
				user := tx.Statement.Dest.(*model.User)
				user.ID = userID
				user.Password = string(oldHash)
				user.SessionVersion = 1
				tx.RowsAffected = 1
			}); err != nil {
				t.Fatal(err)
			}
			updates := 0
			var savedHash string
			if err := db.Callback().Update().Replace("gorm:update", func(tx *gorm.DB) {
				updates++
				savedHash = tx.Statement.Dest.(map[string]interface{})["password"].(string)
				if _, ok := tx.Statement.Dest.(map[string]interface{})["session_version"].(clause.Expr); !ok {
					t.Fatal("password update did not include an atomic session version increment")
				}
				tx.RowsAffected = test.affected
				if test.writeError != nil {
					tx.AddError(test.writeError)
				}
			}); err != nil {
				t.Fatal(err)
			}
			err = NewUserRepository(db).ResetPassword(context.Background(), userID, test.password, test.expectedVersion)
			if (err != nil) != test.wantError || updates != test.wantUpdates {
				t.Fatalf("error = %v, updates = %d, want error = %v, updates = %d", err, updates, test.wantError, test.wantUpdates)
			}
			if test.writeError != nil && !errors.Is(err, test.writeError) {
				t.Fatalf("error = %v, want %v", err, test.writeError)
			}
			if (test.expectedVersion != 1 || test.affected == 0) && !errors.Is(err, ErrSessionChanged) {
				t.Fatalf("got %v, want session conflict", err)
			}
			if !test.wantError && bcrypt.CompareHashAndPassword([]byte(savedHash), []byte(test.password)) != nil {
				t.Fatal("stored hash does not match the new password")
			}
		})
	}
}
