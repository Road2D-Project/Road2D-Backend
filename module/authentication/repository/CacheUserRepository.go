package repository

import (
	"Road-To-Destination-BE/module/authentication/model"
	"context"
	"encoding/json"
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
	if cache.client != nil {
		if user, ok := cache.freshCachedUser(ctx, username); ok {
			return user, nil
		}
	}
	var user model.User
	if err := cache.db.WithContext(ctx).Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	if cache.client == nil {
		return &user, nil
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

// freshCachedUser accepts the Redis row only when its id is still the users row.
// Recreating an account keeps the username and replaces the id. The old cache
// would then list trips for a person who no longer holds those seats.
func (cache *CacheUserRepository) freshCachedUser(ctx context.Context, username string) (*model.User, bool) {
	bytes, err := cache.client.Get(ctx, username).Bytes()
	if err != nil {
		return nil, false
	}
	var record cachedUser
	if json.Unmarshal(bytes, &record) != nil || record.ID == uuid.Nil {
		_ = cache.client.Del(ctx, username).Err()
		return nil, false
	}
	var current model.User
	err = cache.db.WithContext(ctx).Select("id").Where("username = ?", username).Take(&current).Error
	if err != nil || current.ID != record.ID {
		_ = cache.client.Del(ctx, username).Err()
		return nil, false
	}
	return record.toModel(), true
}
