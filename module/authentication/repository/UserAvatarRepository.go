package repository

import (
	"context"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/objectStorage"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const userAvatarBucket = "avatar"

// UserAvatarPaths is the object key for a user's avatar.
// The key is <userID>/<userID><ext> inside the avatar bucket.
type UserAvatarPaths struct{}

func (UserAvatarPaths) Bucket() string { return userAvatarBucket }

func (UserAvatarPaths) ObjectPath(objectID, ext string) string {
	return objectID + "/" + objectID + ext
}

// UserAvatarRepository stores the public avatar URL and satisfies objectStorage.ImageRepo.
type UserAvatarRepository struct {
	db     *gorm.DB
	cache  *redis.Client
	images *objectStorage.Client
}

var _ objectStorage.ImageRepo = (*UserAvatarRepository)(nil)

func NewUserAvatarRepository(db *gorm.DB, cache *redis.Client) *UserAvatarRepository {
	return &UserAvatarRepository{db: db, cache: cache, images: objectStorage.NewClient()}
}

func (r *UserAvatarRepository) SetImagePath(ctx context.Context, objectID, newPath string) (string, error) {
	user, err := r.user(ctx, objectID)
	if err != nil {
		return "", err
	}
	newURL, err := r.images.PublicObjectURL(userAvatarBucket, newPath)
	if err != nil {
		return "", err
	}
	oldKey := ""
	if user.AvatarURL != nil {
		if key, ok := objectStorage.PublicObjectPath(*user.AvatarURL, userAvatarBucket); ok {
			oldKey = key
		}
	}
	err = r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", user.ID).Update("avatar_url", newURL).Error
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
	if user.AvatarURL == nil || *user.AvatarURL == "" {
		return "", nil
	}
	key, ok := objectStorage.PublicObjectPath(*user.AvatarURL, userAvatarBucket)
	if !ok {
		return "", objectStorage.ErrInvalidPath
	}
	return key, nil
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
