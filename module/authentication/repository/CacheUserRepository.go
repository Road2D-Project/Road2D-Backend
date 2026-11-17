package repository

import (
	"context"
	"log"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Only retire a legacy user record belonging to this account. Old keys used
// usernames directly, so an arbitrary username can overlap another Redis key.
var deleteLegacyCredential = redis.NewScript(`
if redis.call('TYPE', KEYS[1]).ok ~= 'string' then
    return 0
end
local raw = redis.call('GET', KEYS[1])
local ok, record = pcall(cjson.decode, raw)
if ok and type(record) == 'table' and record.id == ARGV[1]
   and record.username == ARGV[2] and type(record.password) == 'string' then
    return redis.call('DEL', KEYS[1])
end
return 0
`)

// CacheUserRepository retains the existing adapter API while retiring the old
// username-keyed credential cache. Credentials always come from the database.
type CacheUserRepository struct {
	db     *gorm.DB
	client *redis.Client
}

func NewCacheUserRepository(db *gorm.DB, client *redis.Client) *CacheUserRepository {
	return &CacheUserRepository{db: db, client: client}
}

func (cache *CacheUserRepository) FindUserByUsername(ctx context.Context, username string) (*model.User, error) {
	user, err := NewUserRepository(cache.db).FindUserByUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	cache.clearLegacyCredential(ctx, user)
	return user, nil
}

func (cache *CacheUserRepository) ResetPassword(ctx context.Context, userID uuid.UUID, password string, expectedVersion int64) error {
	repo := NewUserRepository(cache.db)
	user, err := repo.FindUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := repo.ResetPassword(ctx, userID, password, expectedVersion); err != nil {
		return err
	}
	cache.clearLegacyCredential(ctx, user)
	return nil
}

func (cache *CacheUserRepository) clearLegacyCredential(ctx context.Context, user *model.User) {
	if cache.client == nil {
		return
	}
	// A subsequent login retries cleanup and still reads fresh credentials.
	if err := deleteLegacyCredential.Run(ctx, cache.client, []string{user.Username}, user.ID.String(), user.Username).Err(); err != nil {
		log.Printf("auth: legacy credential cache cleanup failed: %v", err)
	}
}
