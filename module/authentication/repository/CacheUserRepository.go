package repository

import (
	"Road-To-Destination-BE/module/authentication/model"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// cachedUser keeps the password hash in Redis. model.User omits Password from JSON.
type cachedUser struct {
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Password  string    `json:"password"`
	CreatedAt time.Time `json:"createdTime"`
	UpdatedAt time.Time `json:"updatedTime"`
}

func cachedUserFromModel(user *model.User) cachedUser {
	return cachedUser{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Password:  user.Password,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (record cachedUser) toModel() *model.User {
	user := &model.User{
		Username: record.Username,
		Email:    record.Email,
		Password: record.Password,
	}
	user.ID = record.ID
	user.CreatedAt = record.CreatedAt
	user.UpdatedAt = record.UpdatedAt
	return user
}

type CacheUserRepository struct {
	db     *gorm.DB
	client *redis.Client
}

func (cache *CacheUserRepository) ResetPassword(ctx context.Context, userId uuid.UUID, newPassword string) error {
	userRepo := NewUserRepository(cache.db)
	user, err := userRepo.FindUserByID(ctx, userId)
	if err != nil {
		return err
	}
	if err := userRepo.ResetPassword(ctx, userId, newPassword); err != nil {
		return err
	}
	_ = cache.client.Del(ctx, user.Username).Err()
	return nil
}

func NewCacheUserRepository(db *gorm.DB, client *redis.Client) *CacheUserRepository {
	return &CacheUserRepository{db: db, client: client}
}

func (cache *CacheUserRepository) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	bytes, err := cache.client.Get(ctx, username).Bytes()
	if errors.Is(err, redis.Nil) {
		var user model.User
		if err := cache.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
			return nil, err
		}
		payload, err := json.Marshal(cachedUserFromModel(&user))
		if err != nil {
			return nil, err
		}
		if err := cache.client.Set(ctx, username, payload, 0).Err(); err != nil {
			return nil, err
		}
		return &user, nil
	}
	if err != nil {
		return nil, err
	}
	var record cachedUser
	if err := json.Unmarshal(bytes, &record); err != nil {
		return nil, err
	}
	return record.toModel(), nil
}
