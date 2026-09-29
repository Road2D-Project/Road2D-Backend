package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func testResponseCache(t *testing.T) *ResponseCache {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewResponseCache(client)
}

func TestCacheAroundStoresThenServes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := testResponseCache(t)
	hits := 0
	router := gin.New()
	router.GET("/trips/:tripId/graph", putGraphKey, cache.CacheAround(), func(c *gin.Context) {
		hits++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	first := getGraph(router)
	second := getGraph(router)
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status %d then %d", first.Code, second.Code)
	}
	if hits != 1 {
		t.Fatalf("handler ran %d times, want 1", hits)
	}
	if second.Body.String() != first.Body.String() {
		t.Fatalf("cached body %s, first was %s", second.Body.String(), first.Body.String())
	}
}

func TestMarkChangedForcesTheNextRead(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := testResponseCache(t)
	hits := 0
	router := gin.New()
	router.GET("/trips/:tripId/graph", putGraphKey, cache.CacheAround(), func(c *gin.Context) {
		hits++
		c.JSON(http.StatusOK, gin.H{"n": hits})
	})
	router.PUT("/trips/:tripId/graph", putGraphKey, cache.MarkChanged(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	_ = getGraph(router)
	put := httptest.NewRequest(http.MethodPut, "/trips/trip-1/graph", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, put)
	if recorder.Code != http.StatusOK {
		t.Fatalf("put status %d", recorder.Code)
	}
	_ = getGraph(router)
	if hits != 2 {
		t.Fatalf("handler ran %d times, want 2", hits)
	}
}

func TestCacheAroundSkipsSaveWhenAWriteLandsMidRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := testResponseCache(t)
	router := gin.New()
	router.GET("/trips/:tripId/graph", putGraphKey, cache.CacheAround(), func(c *gin.Context) {
		// Cùng việc MarkChanged làm: đặt dấu changed rồi tăng generation, trong lúc GET này chưa ghi xong.
		ctx := c.Request.Context()
		if err := cache.client.Set(ctx, "resp:graph:trip-1", cacheChanged, cache.ttl).Err(); err != nil {
			t.Fatal(err)
		}
		if err := cache.client.Incr(ctx, generationKey("resp:graph:trip-1")).Err(); err != nil {
			t.Fatal(err)
		}
		c.JSON(http.StatusOK, gin.H{"stale": true})
	})

	_ = getGraph(router)
	raw, err := cache.client.Get(context.Background(), "resp:graph:trip-1").Result()
	if err != nil {
		t.Fatal(err)
	}
	if raw != cacheChanged {
		t.Fatalf("body = %q, want the changed mark", raw)
	}
}

func TestCacheAroundWithoutRedisStillRunsTheHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := NewResponseCache(nil)
	hits := 0
	router := gin.New()
	router.GET("/trips/:tripId/graph", putGraphKey, cache.CacheAround(), func(c *gin.Context) {
		hits++
		c.Status(http.StatusOK)
	})
	if getGraph(router).Code != http.StatusOK || hits != 1 {
		t.Fatalf("hits = %d", hits)
	}
}

func putGraphKey(c *gin.Context) {
	c.Set(MainKeyController, []string{"resp:graph:" + c.Param("tripId")})
	c.Next()
}

func getGraph(router *gin.Engine) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/trips/trip-1/graph", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}
