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
	router.PUT("/trips/:tripId/graph", putGraphKey, cache.MarkChanged(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/trips/:tripId/graph", putGraphKey, cache.CacheAround(), func(c *gin.Context) {
		// Finish a real write while this GET still holds the old generation.
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPut, "/trips/trip-1/graph", nil))
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("write status = %d", recorder.Code)
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

type cacheInvalidationTestKey struct{}

// Simulate an old GET saving immediately after the first invalidation command.
// Separate SET/INCR commands allow that body to survive the invalidation.
type staleResponseSaveHook struct {
	cache     *ResponseCache
	attempted bool
	saved     int
	err       error
}

func (h *staleResponseSaveHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *staleResponseSaveHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return next
}

func (h *staleResponseSaveHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		err := next(ctx, cmd)
		if err == nil && ctx.Value(cacheInvalidationTestKey{}) == true && !h.attempted {
			h.attempted = true
			h.saved, h.err = saveBody.Run(context.Background(), h.cache.client,
				[]string{"resp:graph:trip-1", generationKey("resp:graph:trip-1")},
				"0", `{"stale":true}`, int(h.cache.ttl.Seconds())).Int()
		}
		return err
	}
}

func TestMarkChangedPreventsStaleSaveBetweenRedisCommands(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cache := testResponseCache(t)
	// Initialize the connection before installing the command interleaving hook.
	if err := cache.client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	hook := &staleResponseSaveHook{cache: cache}
	cache.client.AddHook(hook)
	router := gin.New()
	router.PUT("/trips/:tripId/graph", putGraphKey, cache.MarkChanged(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	ctx := context.WithValue(context.Background(), cacheInvalidationTestKey{}, true)
	request := httptest.NewRequest(http.MethodPut, "/trips/trip-1/graph", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	if !hook.attempted || hook.err != nil || hook.saved != 0 {
		t.Fatalf("stale save attempted = %v, saved = %d, error = %v", hook.attempted, hook.saved, hook.err)
	}
	raw, err := cache.client.Get(context.Background(), "resp:graph:trip-1").Result()
	if err != nil || raw != cacheChanged {
		t.Fatalf("cached body = %q, error = %v", raw, err)
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
