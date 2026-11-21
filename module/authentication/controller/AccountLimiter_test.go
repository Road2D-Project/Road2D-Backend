package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Road-To-Destination-BE/middleware"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type deniedAccountLimiter struct{ keys []string }

func (l *deniedAccountLimiter) Allow(_ context.Context, key string) (middleware.TokenBucketResult, error) {
	l.keys = append(l.keys, key)
	return middleware.TokenBucketResult{}, nil
}

func (*deniedAccountLimiter) RequireLimiter() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func (*deniedAccountLimiter) RequireIPLimiter() gin.HandlerFunc {
	return func(c *gin.Context) { c.Next() }
}

func TestLoginLimitsAccountAcrossIPsBeforeCredentialLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &deniedAccountLimiter{}
	router := gin.New()
	NewAuthenticationController(nil, nil, nil, limiter, nil).RegisterRoutes(router.Group("/v1"))
	for _, ip := range []string{"192.0.2.1", "192.0.2.2"} {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/user/login", strings.NewReader(`{"username":"traveler","password":"TestPassword1!"}`))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = ip + ":1234"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("IP %s returned %d, want account limit", ip, recorder.Code)
		}
	}
	if len(limiter.keys) != 2 || limiter.keys[0] != limiter.keys[1] || strings.Contains(limiter.keys[0], "traveler") {
		t.Fatalf("account keys were not shared and hashed: %v", limiter.keys)
	}
}

func TestForgetPasswordLimitsEmailAcrossIPsBeforeLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limiter := &deniedAccountLimiter{}
	router := gin.New()
	NewAuthenticationController(nil, nil, nil, limiter, nil).RegisterRoutes(router.Group("/v1"))
	for _, test := range []struct {
		email string
		ip    string
	}{
		{"traveler@example.com", "192.0.2.1"},
		{"traveler@example.com", "192.0.2.2"},
		{"another@example.com", "192.0.2.3"},
	} {
		body := `{"mail":"` + test.email + `"}`
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/user/forget-password", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = test.ip + ":1234"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusTooManyRequests {
			t.Fatalf("email %s from IP %s returned %d", test.email, test.ip, recorder.Code)
		}
	}
	if len(limiter.keys) != 3 || limiter.keys[0] != limiter.keys[1] || limiter.keys[0] == limiter.keys[2] {
		t.Fatalf("limiter did not group requests by email: %v", limiter.keys)
	}
	for _, key := range limiter.keys {
		if strings.Contains(key, "@") {
			t.Fatalf("email was exposed in limiter key %q", key)
		}
	}
}

func TestAuthRoutesApplyIPLimitsIncludingResetTokenPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("SECRET_MAIL_JWT", "mail-test-secret-01234567890123456789")
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	limiter, err := middleware.NewTokenBucket(middleware.TokenBucketConfig{
		Capacity: 1, RefillRate: 1, RefillInterval: time.Hour,
	}, client)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	NewAuthenticationController(nil, client, nil, limiter, nil).RegisterRoutes(router.Group("/v1"))
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/v1/auth/user/login"},
		{http.MethodPost, "/v1/auth/user/register"},
		{http.MethodPost, "/v1/auth/user/refresh"},
		{http.MethodPost, "/v1/auth/user/logout"},
		{http.MethodPost, "/v1/auth/user/forget-password"},
		{http.MethodGet, "/v1/auth/user/reset-password/invalid-a"},
		{http.MethodPost, "/v1/auth/user/reset-password/invalid-a"},
	} {
		for _, want := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
			path := route.path
			if want == http.StatusTooManyRequests {
				path = strings.ReplaceAll(path, "invalid-a", "invalid-b")
			}
			request := httptest.NewRequest(route.method, path, strings.NewReader("{"))
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)
			if recorder.Code != want {
				t.Fatalf("%s %s returned %d, want %d", route.method, path, recorder.Code, want)
			}
		}
	}
}
