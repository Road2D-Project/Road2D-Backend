package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func testTokenBucket(t *testing.T, capacity int) (RateLimiter, *redis.Client) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	limiter, err := NewTokenBucket(TokenBucketConfig{
		Capacity: capacity, RefillRate: 1, RefillInterval: time.Hour,
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	return limiter, client
}

func TestTokenBucketConsumesCapacityAndReturnsRemainingTokens(t *testing.T) {
	limiter, _ := testTokenBucket(t, 2)
	for index, want := range []TokenBucketResult{
		{Allowed: true, RemainingTokens: 1},
		{Allowed: true, RemainingTokens: 0},
		{Allowed: false, RemainingTokens: 0},
	} {
		got, err := limiter.Allow(context.Background(), "bucket")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("request %d returned %+v, want %+v", index+1, got, want)
		}
	}
}

func TestTokenBucketRefillsWithoutExceedingCapacity(t *testing.T) {
	limiter, client := testTokenBucket(t, 2)
	ctx := context.Background()
	if err := client.HSet(ctx, "bucket", "tokens", 0,
		"last_refill", time.Now().Add(-3*time.Hour).UnixMilli()).Err(); err != nil {
		t.Fatal(err)
	}
	result, err := limiter.Allow(ctx, "bucket")
	if err != nil {
		t.Fatal(err)
	}
	if result != (TokenBucketResult{Allowed: true, RemainingTokens: 1}) {
		t.Fatalf("refilled bucket returned %+v", result)
	}
	ttl, err := client.PTTL(ctx, "bucket").Result()
	if err != nil || ttl <= 0 || ttl > 2*time.Hour {
		t.Fatalf("bucket TTL = %s, error = %v", ttl, err)
	}
}

func TestIPLimiterSeparatesRoutesAndIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, _ := testTokenBucket(t, 1)
	router := gin.New()
	handler := func(c *gin.Context) { c.Status(http.StatusNoContent) }
	router.POST("/auth/login", limiter.RequireIPLimiter(), handler)
	router.POST("/auth/register", limiter.RequireIPLimiter(), handler)
	router.GET("/serp/:placeId", limiter.RequireIPLimiter(), handler)

	for _, test := range []struct {
		method string
		path   string
		ip     string
		status int
	}{
		{http.MethodPost, "/auth/login", "192.0.2.1", http.StatusNoContent},
		{http.MethodPost, "/auth/login", "192.0.2.1", http.StatusTooManyRequests},
		{http.MethodPost, "/auth/register", "192.0.2.1", http.StatusNoContent},
		{http.MethodGet, "/serp/place-a", "192.0.2.1", http.StatusNoContent},
		{http.MethodGet, "/serp/place-b", "192.0.2.1", http.StatusTooManyRequests},
		{http.MethodPost, "/auth/login", "192.0.2.2", http.StatusNoContent},
	} {
		request := httptest.NewRequest(test.method, test.path, nil)
		request.RemoteAddr = test.ip + ":1234"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("%s %s from %s returned %d, want %d", test.method, test.path, test.ip, recorder.Code, test.status)
		}
		if test.status == http.StatusTooManyRequests && !json.Valid(recorder.Body.Bytes()) {
			t.Fatalf("invalid rate-limit response: %s", recorder.Body.String())
		}
	}
}

func TestUserLimiterRequiresAuthenticationAndSeparatesUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, _ := testTokenBucket(t, 1)
	userID := uuid.New()
	otherUserID := uuid.New()
	router := gin.New()
	router.GET("/resource", func(c *gin.Context) {
		if id := c.GetHeader("Test-User-ID"); id != "" {
			user := &model.User{}
			user.ID = uuid.MustParse(id)
			c.Set("currentUser", user)
		}
		c.Next()
	}, limiter.RequireLimiter(), func(c *gin.Context) { c.Status(http.StatusNoContent) })

	for _, test := range []struct {
		userID string
		status int
	}{
		{"", http.StatusUnauthorized},
		{userID.String(), http.StatusNoContent},
		{userID.String(), http.StatusTooManyRequests},
		{otherUserID.String(), http.StatusNoContent},
	} {
		request := httptest.NewRequest(http.MethodGet, "/resource", nil)
		request.Header.Set("Test-User-ID", test.userID)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("user %q returned %d, want %d", test.userID, recorder.Code, test.status)
		}
	}
}

func TestIPLimiterStopsRequestWhenRedisIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter, client := testTokenBucket(t, 1)
	if err := client.Close(); err != nil {
		t.Fatal(err)
	}
	handled := false
	router := gin.New()
	router.GET("/resource", limiter.RequireIPLimiter(), func(c *gin.Context) {
		handled = true
		c.Status(http.StatusNoContent)
	})
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/resource", nil))
	if recorder.Code != http.StatusServiceUnavailable || handled {
		t.Fatalf("status = %d, handler ran = %v", recorder.Code, handled)
	}
}

func TestTokenBucketConcurrentRequestsRespectCapacity(t *testing.T) {
	limiter, _ := testTokenBucket(t, 10)
	var allowed atomic.Int64
	var workers sync.WaitGroup
	for range 40 {
		workers.Go(func() {
			result, err := limiter.Allow(context.Background(), "shared-bucket")
			if err != nil {
				t.Error(err)
				return
			}
			if result.Allowed {
				allowed.Add(1)
			}
		})
	}
	workers.Wait()
	if allowed.Load() != 10 {
		t.Fatalf("allowed %d requests, want 10", allowed.Load())
	}
}
