package client

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/share"

	"github.com/redis/go-redis/v9"
)

// Daily usage follows the Redis server's UTC day across all application instances.
var consumeDailyQuota = redis.NewScript(`
local now = redis.call('TIME')
local day = math.floor(tonumber(now[1]) / 86400)
local state = redis.call('HMGET', KEYS[1], 'day', 'used')
local used = 0
if tonumber(state[1]) == day then
    used = tonumber(state[2]) or 0
end
if used >= tonumber(ARGV[1]) then
    return 0
end
redis.call('HSET', KEYS[1], 'day', day, 'used', used + 1)
redis.call('EXPIREAT', KEYS[1], (day + 1) * 86400)
return 1
`)

// ProviderQuota counts outgoing attempts, including upstream failures that may
// have consumed provider quota. Cache hits never reach this check.
type ProviderQuota struct {
	client     *redis.Client
	limiter    middleware.RateLimiter
	key        string
	dailyLimit int
}

func NewProviderQuota(provider string, client *redis.Client) (*ProviderQuota, error) {
	config := middleware.DefaultTokenBucketConfig()
	var defaultDailyLimit int
	switch provider {
	case "goong":
		config.Capacity = freeRateLimit
		config.RefillRate = freeRateLimit
		defaultDailyLimit = 10000
	case "serp":
		defaultDailyLimit = 100
	default:
		return nil, fmt.Errorf("unsupported map provider %q", provider)
	}
	envKey := strings.ToUpper(provider) + "_DAILY_REQUEST_LIMIT"
	dailyLimit, err := strconv.Atoi(share.GetEnvStringDefault(envKey, strconv.Itoa(defaultDailyLimit)))
	if err != nil || dailyLimit <= 0 {
		return nil, fmt.Errorf("%s must be a positive integer", envKey)
	}
	limiter, err := middleware.NewTokenBucket(config, client)
	if err != nil {
		return nil, err
	}
	return &ProviderQuota{
		client: client, limiter: limiter, key: "quota:maps:" + provider, dailyLimit: dailyLimit,
	}, nil
}

func (q *ProviderQuota) Allow(ctx context.Context) error {
	result, err := q.limiter.Allow(ctx, q.key+":rate")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQuotaUnavailable, err)
	}
	if !result.Allowed {
		return ErrProviderQuotaExceeded
	}
	allowed, err := consumeDailyQuota.Run(ctx, q.client, []string{q.key + ":daily"}, q.dailyLimit).Int()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrQuotaUnavailable, err)
	}
	if allowed == 0 {
		return ErrProviderQuotaExceeded
	}
	return nil
}
