package middleware

import (
	"Road-To-Destination-BE/module/share"
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type RateLimiter interface {
	Allow(ctx context.Context, key string) (TokenBucketResult, error)
	RequireLimiter() gin.HandlerFunc
	RequireIPLimiter() gin.HandlerFunc
}

const TokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local refill_interval = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Get current state or initialize
local bucket = redis.call('HMGET', key, 'tokens', 'last_refill')
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

-- Initialize if this is the first request
if tokens == nil then
    tokens = capacity
    last_refill = now
end

-- Calculate token refill
local time_passed = now - last_refill
local refills = math.floor(time_passed / refill_interval)

if refills > 0 then
    tokens = math.min(capacity, tokens + (refills * refill_rate))
    last_refill = last_refill + (refills * refill_interval)
end

-- Try to consume a token
local allowed = 0
if tokens >= 1 then
    tokens = tokens - 1
    allowed = 1
end

-- Update state
redis.call('HSET', key, 'tokens', tokens, 'last_refill', last_refill)

-- Return result: allowed (1 or 0) and remaining tokens
local ttl = math.ceil(capacity / refill_rate) * refill_interval
redis.call('PEXPIRE', key, ttl)

return {allowed, tokens}
`

type TokenBucketConfig struct {
	Capacity       int
	RefillRate     int
	RefillInterval time.Duration
}
type TokenBucketLimiter struct {
	Scripter *redis.Script
	Client   *redis.Client
	config   TokenBucketConfig
}

func (t TokenBucketLimiter) RequireLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := GetCurrentUser(c)
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, share.NewError(http.StatusUnauthorized, "Unauthorized"))
			return
		}
		key := "rl:api:user:" + c.Request.Method + ":" + c.FullPath() + ":" + user.ID.String()
		t.handleLimiter(c, key)
	}
}
func (t TokenBucketLimiter) RequireIPLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := "rl:api:ip:" + c.Request.Method + ":" + c.FullPath() + ":" + ip
		t.handleLimiter(c, key)
	}
}
func (t TokenBucketLimiter) handleLimiter(c *gin.Context, key string) {
	token, err := t.Allow(c.Request.Context(), key)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, share.NewError(http.StatusServiceUnavailable, "Rate limiter temporarily unavailable"))
		return
	}
	if !token.Allowed {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, share.NewError(http.StatusTooManyRequests, "Too many requests"))
		return
	}
	c.Next()
}

type TokenBucketResult struct {
	Allowed         bool
	RemainingTokens int
}

func (t TokenBucketLimiter) Allow(ctx context.Context, key string) (TokenBucketResult, error) {
	raw, err := t.Scripter.Run(
		ctx,
		t.Client,
		[]string{key},
		t.config.Capacity,                      // ARGV[1]
		t.config.RefillRate,                    // ARGV[2]
		t.config.RefillInterval.Milliseconds(), // ARGV[3]
		time.Now().UnixMilli(),                 // ARGV[4]
	).Result()
	if err != nil {
		return TokenBucketResult{}, fmt.Errorf(
			"run token bucket script: %w", err,
		)
	}
	values, ok := raw.([]interface{})
	if !ok || len(values) != 2 {
		return TokenBucketResult{}, errors.New(
			"unexpected token bucket response",
		)
	}

	allowed, okAllowed := values[0].(int64)
	remaining, okRemaining := values[1].(int64)
	if !okAllowed || !okRemaining {
		return TokenBucketResult{}, errors.New(
			"invalid token bucket response types",
		)
	}
	return TokenBucketResult{Allowed: allowed == 1, RemainingTokens: int(remaining)}, nil
}
func NewTokenBucket(config TokenBucketConfig, client *redis.Client) (RateLimiter, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if config.Capacity <= 0 ||
		config.RefillRate <= 0 ||
		config.RefillInterval < time.Millisecond {
		return nil, errors.New("invalid token bucket config")
	}
	tokenLimiter := &TokenBucketLimiter{
		Scripter: redis.NewScript(TokenBucketScript),
		config:   config,
		Client:   client,
	}

	return tokenLimiter, nil
}
func DefaultTokenBucketConfig() TokenBucketConfig {
	return TokenBucketConfig{
		Capacity:       10,
		RefillRate:     1,
		RefillInterval: time.Second,
	}
}
