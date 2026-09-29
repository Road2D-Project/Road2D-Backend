package middleware

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

const (
	// Sentinel values stored in the body key instead of real JSON.
	// A key that was never SET is treated the same as cacheNoData.
	cacheNoData  = "no data" // nothing has been cached yet
	cacheChanged = "changed" // a write happened; the old cached body is stale

	cacheTTL = 15 * time.Minute

	// MainKeyController is the gin.Context key each route uses to register
	// the Redis key(s) its response depends on. The middleware itself never
	// builds these keys (no string concatenation of tripId, resource name,
	// etc.) — only the route knows what its own response is keyed by.
	MainKeyController = "mainKeyController"
)

// saveBody stores a freshly computed body, but ONLY if no write has bumped
// the generation counter since this request started reading.
//
// Why this matters: a GET request may take a while to compute its response
// (DB queries, joins, etc). If a PUT/POST changes the underlying data WHILE
// that GET is still computing, the GET's result is already stale by the time
// it finishes — saving it to cache would "resurrect" old data over the
// "changed" marker the write just set. This script prevents that race:
//
//	KEYS[1]  = body key                (where the cached JSON lives)
//	KEYS[2]  = generation key          (a counter, bumped on every write)
//	ARGV[1]  = generation value this request observed before it started
//	ARGV[2]  = the body to cache
//	ARGV[3]  = TTL in seconds
//
// Returns 1 if the write happened, 0 if it was skipped as stale.
var saveBody = redis.NewScript(`
local current = redis.call('GET', KEYS[2])
if current == false then
	current = '0'
end
if current ~= ARGV[1] then
	return 0
end
redis.call('SET', KEYS[1], ARGV[2], 'EX', ARGV[3])
return 1
`)

// ResponseCache caches the JSON body of GET responses and lets write
// requests (POST/PUT) invalidate them. If Redis is unavailable, every
// method becomes a no-op and requests just fall through to their handlers.
type ResponseCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewResponseCache(client *redis.Client) *ResponseCache {
	return &ResponseCache{client: client, ttl: cacheTTL}
}

func (cache *ResponseCache) enabled() bool {
	return cache != nil && cache.client != nil
}

// CacheAround wraps a GET handler.
//
//   - Cache hit (valid JSON, not a sentinel) → respond immediately, abort
//     the chain, the real handler never runs.
//   - Cache miss / "no data" / "changed" → run the handler, then try to
//     save its response — but only if the response was a plain 200 AND no
//     write happened in the meantime (see saveBody above).
//
// Any Redis error is treated as "just run the handler normally" — caching
// must never turn a working GET into a 500.
func (cache *ResponseCache) CacheAround() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !cache.enabled() {
			c.Next()
			return
		}

		keys, ok := cacheKeys(c)
		if !ok {
			c.Next()
			return
		}
		bodyKey := keys[0] // a GET is keyed by a single Redis key
		ctx := c.Request.Context()

		if cached, hit := cache.tryServeFromCache(ctx, c, bodyKey); hit {
			_ = cached // response already written by tryServeFromCache
			return
		}

		// Snapshot the generation counter BEFORE running the handler.
		// This is the "version" saveBody will later check against.
		generationSeen := cache.readGeneration(ctx, bodyKey)

		// Capture what the handler writes so we can cache it afterwards,
		// while still streaming it to the real client as normal.
		capture := &bodyCapture{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = capture
		c.Next()

		if capture.Status() != http.StatusOK || capture.body.Len() == 0 {
			return // don't cache errors or empty bodies
		}

		_, _ = saveBody.Run(
			ctx,
			cache.client,
			[]string{bodyKey, generationKey(bodyKey)},
			generationSeen,
			capture.body.String(),
			int(cache.ttl.Seconds()),
		).Result()
	}
}

// tryServeFromCache checks Redis for a usable cached body and, if found,
// writes it directly to the response and returns true.
func (cache *ResponseCache) tryServeFromCache(ctx context.Context, c *gin.Context, bodyKey string) (string, bool) {
	raw, err := cache.client.Get(ctx, bodyKey).Result()
	if err != nil || raw == cacheNoData || raw == cacheChanged {
		return "", false
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(raw))
	c.Abort()
	return raw, true
}

// MarkChanged wraps a POST or PUT handler. It runs AFTER the handler
// (c.Next() first), so it only marks the cache as stale once the write has
// actually succeeded (2xx).
//
// For every affected key it does two things, in this exact order:
//  1. SET the body key to "changed"
//  2. INCR the generation counter
//
// Order matters: setting "changed" first means that if a GET is mid-flight
// and finishes right after, saveBody's generation check will already see
// the bumped counter and refuse to overwrite the "changed" marker with a
// stale body.
func (cache *ResponseCache) MarkChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if !cache.enabled() {
			return
		}

		status := c.Writer.Status()
		if status < 200 || status >= 300 {
			return // failed write: leave the existing cache untouched
		}

		keys, ok := cacheKeys(c)
		if !ok {
			return
		}

		ctx := c.Request.Context()
		for _, bodyKey := range keys {
			if err := cache.client.Set(ctx, bodyKey, cacheChanged, cache.ttl).Err(); err != nil {
				return
			}
			if err := cache.client.Incr(ctx, generationKey(bodyKey)).Err(); err != nil {
				return
			}
		}
	}
}

// readGeneration returns the current generation counter for a body key,
// defaulting to "0" if it doesn't exist yet — the same default the Lua
// script uses when GET returns false.
func (cache *ResponseCache) readGeneration(ctx context.Context, bodyKey string) string {
	value, err := cache.client.Get(ctx, generationKey(bodyKey)).Result()
	if err != nil || value == "" {
		return "0"
	}
	return value
}

// generationKey derives the counter key from its body key, e.g.
// "cache:trip:42" -> "cache:trip:42:gen".
func generationKey(bodyKey string) string {
	return bodyKey + ":gen"
}

// cacheKeys reads the Redis key(s) the route registered on the context.
//   - GET uses only keys[0]: one response, one cache entry.
//   - POST/PUT may register several keys, so a single write can invalidate
//     multiple GET responses at once (e.g. a list view and a detail view).
func cacheKeys(c *gin.Context) ([]string, bool) {
	raw, ok := c.Get(MainKeyController)
	if !ok {
		return nil, false
	}
	keys, ok := raw.([]string)
	return keys, ok && len(keys) > 0
}

// bodyCapture is a gin.ResponseWriter wrapper that duplicates every write
// into an in-memory buffer, so the handler's output can be cached AFTER
// it's already been streamed to the real client.
type bodyCapture struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyCapture) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyCapture) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}
