package repository

import (
	"context"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/objectStorage"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// UserAvatarRepository stores the avatar key and satisfies objectStorage.ImageRepo.
type UserAvatarRepository struct {
	db    *gorm.DB
	cache *redis.Client
}

var _ objectStorage.ImageRepo = (*UserAvatarRepository)(nil)

func NewUserAvatarRepository(db *gorm.DB, cache *redis.Client) *UserAvatarRepository {
	return &UserAvatarRepository{db: db, cache: cache}
}

func (r *UserAvatarRepository) SetImagePath(ctx context.Context, objectID, newPath string) (string, error) {
	user, err := r.user(ctx, objectID)
	if err != nil {
		return "", err
	}
	if err := objectStorage.ValidateOwnedPath(objectID, newPath); err != nil {
		return "", err
	}
	oldKey := ""
	if user.AvatarPath != nil {
		oldKey = *user.AvatarPath
	}
	err = r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("avatar_url", newPath).Error
	if err != nil {
		return "", err
	}
	r.forget(ctx, user.Username)
	return oldKey, nil
}

func (r *UserAvatarRepository) ImagePath(ctx context.Context, objectID string) (string, error) {
	user, err := r.user(ctx, objectID)
	if err != nil {
		return "", err
	}
	if user.AvatarPath == nil || *user.AvatarPath == "" {
		return "", nil
	}
	if err := objectStorage.ValidateOwnedPath(objectID, *user.AvatarPath); err != nil {
		return "", err
	}
	return *user.AvatarPath, nil
}

func (r *UserAvatarRepository) ClearImagePath(ctx context.Context, objectID string) error {
	user, err := r.user(ctx, objectID)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("avatar_url", nil).Error
	if err != nil {
		return err
	}
	r.forget(ctx, user.Username)
	return nil
}

func (r *UserAvatarRepository) user(ctx context.Context, objectID string) (*model.User, error) {
	id, err := uuid.Parse(objectID)
	if err != nil {
		return nil, err
	}
	var user model.User
	err = r.db.WithContext(ctx).Select("id", "username", "avatar_url").Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserAvatarRepository) forget(ctx context.Context, username string) {
	if r.cache == nil || username == "" {
		return
	}
	_ = r.cache.Del(ctx, username).Err()
}
