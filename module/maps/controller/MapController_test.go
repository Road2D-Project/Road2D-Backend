package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Road-To-Destination-BE/middleware"
	"Road-To-Destination-BE/module/maps/client"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func TestGoongPlaygroundAppliesKeyCheckBeforeIPLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("GOONG_PLAYGROUND_KEY", "playground-secret")
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	limiter, err := middleware.NewTokenBucket(middleware.TokenBucketConfig{
		Capacity: 1, RefillRate: 1, RefillInterval: time.Hour,
	}, redisClient)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	playground := router.Group("/v1", middleware.RequireHeaderPlaygroundKey())
	NewMapController(nil, redisClient, client.NewDefaultGoongClient(), limiter).RegisterPlayground(playground)
	for _, test := range []struct {
		key    string
		ip     string
		status int
	}{
		{"", "192.0.2.1", http.StatusUnauthorized},
		{"playground-secret", "192.0.2.1", http.StatusBadRequest},
		{"playground-secret", "192.0.2.1", http.StatusTooManyRequests},
		{"playground-secret", "192.0.2.2", http.StatusBadRequest},
	} {
		// Missing input is rejected by validation before any provider call.
		request := httptest.NewRequest(http.MethodGet, "/v1/goong/places/autocomplete", nil)
		request.Header.Set(middleware.PlaygroundKeyHeader, test.key)
		request.RemoteAddr = test.ip + ":1234"
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatalf("key %q, IP %s returned %d, want %d", test.key, test.ip, recorder.Code, test.status)
		}
	}
}
