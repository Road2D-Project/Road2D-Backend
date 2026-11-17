package repository

import (
	"context"
	"encoding/json"
	"testing"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func testCredentialDatabase(t *testing.T, user *model.User) *gorm.DB {
	t.Helper()
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
	if err := db.Callback().Query().Replace("gorm:query", func(tx *gorm.DB) {
		*tx.Statement.Dest.(*model.User) = *user
		tx.RowsAffected = 1
	}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCredentialsComeFromDatabaseDespiteLegacyCacheOrRedisFailure(t *testing.T) {
	for _, test := range []struct {
		name                string
		cached, redisFailed bool
	}{
		{"stale legacy record", true, false},
		{"failed legacy deletion", true, true},
		{"cache miss", false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			user := &model.User{Username: "traveler", Password: "current-db-hash", SessionVersion: 2}
			user.ID = uuid.New()
			mini := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			if test.cached {
				legacy, err := json.Marshal(map[string]interface{}{
					"id": user.ID, "username": user.Username, "password": "stale-cache-hash",
				})
				if err != nil {
					t.Fatal(err)
				}
				if err := client.Set(context.Background(), user.Username, legacy, 0).Err(); err != nil {
					t.Fatal(err)
				}
			}
			if test.redisFailed {
				_ = client.Close()
			}
			repo := NewCacheUserRepository(testCredentialDatabase(t, user), client)
			got, err := repo.FindUserByUsername(context.Background(), user.Username)
			if err != nil || got.Password != user.Password || got.SessionVersion != 2 {
				t.Fatalf("credential lookup returned %+v, %v", got, err)
			}
			if !test.redisFailed && mini.Exists(user.Username) {
				t.Fatal("legacy credential remained or a new credential cache entry was created")
			}
		})
	}
}

func TestLegacyCleanupPreservesUnrelatedRedisKeys(t *testing.T) {
	user := &model.User{Username: "quota:maps:goong:daily", SessionVersion: 1}
	user.ID = uuid.New()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.HSet(context.Background(), user.Username, "used", 10).Err(); err != nil {
		t.Fatal(err)
	}
	repo := NewCacheUserRepository(testCredentialDatabase(t, user), client)
	if _, err := repo.FindUserByUsername(context.Background(), user.Username); err != nil {
		t.Fatal(err)
	}
	if !mini.Exists(user.Username) {
		t.Fatal("cleanup deleted an unrelated provider quota")
	}
}
