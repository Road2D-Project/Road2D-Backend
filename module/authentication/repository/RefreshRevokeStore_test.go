package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"Road-To-Destination-BE/module/authentication/model"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type memoryRevokePersist struct {
	mu   sync.Mutex
	rows map[string]model.RevokedRefreshToken
}

func newMemoryRevokePersist() *memoryRevokePersist {
	return &memoryRevokePersist{rows: make(map[string]model.RevokedRefreshToken)}
}

func (p *memoryRevokePersist) Upsert(_ context.Context, row *model.RevokedRefreshToken) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.rows[row.TokenHash]; !exists {
		copied := *row
		p.rows[row.TokenHash] = copied
	}
	return nil
}

func (p *memoryRevokePersist) FindActive(_ context.Context, hash string, now time.Time) (*model.RevokedRefreshToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	row, ok := p.rows[hash]
	if !ok || !row.ExpiresAt.After(now) {
		return nil, nil
	}
	copied := row
	return &copied, nil
}

func (p *memoryRevokePersist) DeleteExpired(_ context.Context, now time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for hash, row := range p.rows {
		if !row.ExpiresAt.After(now) {
			delete(p.rows, hash)
		}
	}
	return nil
}

func testRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mini := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return mini, client
}

func TestRevokeThenIsRevokedHitsRedis(t *testing.T) {
	_, client := testRedis(t)
	store := &RefreshRevokeStore{redis: client, persist: newMemoryRevokePersist()}
	ctx := context.Background()
	token := "refresh.jwt.example"
	exp := time.Now().Add(time.Hour)

	if err := store.Revoke(ctx, token, "traveler", exp); err != nil {
		t.Fatal(err)
	}
	revoked, err := store.IsRevoked(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("expected revoked")
	}
}

func TestIsRevokedFallsBackToPersistAfterRedisFlush(t *testing.T) {
	mini, client := testRedis(t)
	persist := newMemoryRevokePersist()
	store := &RefreshRevokeStore{redis: client, persist: persist}
	ctx := context.Background()
	token := "refresh.jwt.example"
	exp := time.Now().Add(2 * time.Hour)

	if err := store.Revoke(ctx, token, "traveler", exp); err != nil {
		t.Fatal(err)
	}
	mini.FlushAll()

	revoked, err := store.IsRevoked(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("persist should still revoke after Redis loses keys")
	}

	hash := HashRefreshToken(token)
	if !mini.Exists(revokedRefreshKey(hash)) {
		t.Fatal("expected Redis backfill after persist hit")
	}
}

func TestRevokeWithoutRedisUsesPersist(t *testing.T) {
	store := &RefreshRevokeStore{persist: newMemoryRevokePersist()}
	ctx := context.Background()
	token := "refresh.jwt.example"
	if err := store.Revoke(ctx, token, "traveler", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	revoked, err := store.IsRevoked(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if !revoked {
		t.Fatal("expected persist-only revoke")
	}
}

func TestExpiredTokenIsNotRevoked(t *testing.T) {
	store := &RefreshRevokeStore{persist: newMemoryRevokePersist()}
	ctx := context.Background()
	token := "refresh.jwt.example"
	if err := store.Revoke(ctx, token, "traveler", time.Now().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	revoked, err := store.IsRevoked(ctx, token)
	if err != nil {
		t.Fatal(err)
	}
	if revoked {
		t.Fatal("expired JWT does not need a denylist row")
	}
}

func TestRevokeIdempotent(t *testing.T) {
	store := &RefreshRevokeStore{persist: newMemoryRevokePersist()}
	ctx := context.Background()
	token := "refresh.jwt.example"
	exp := time.Now().Add(time.Hour)
	if err := store.Revoke(ctx, token, "traveler", exp); err != nil {
		t.Fatal(err)
	}
	if err := store.Revoke(ctx, token, "traveler", exp); err != nil {
		t.Fatal(err)
	}
	if len(store.persist.(*memoryRevokePersist).rows) != 1 {
		t.Fatalf("rows = %d", len(store.persist.(*memoryRevokePersist).rows))
	}
}

func TestHashRefreshTokenStable(t *testing.T) {
	if HashRefreshToken("a") == HashRefreshToken("b") {
		t.Fatal("different tokens must not share a hash")
	}
	if got, want := HashRefreshToken("a"), HashRefreshToken("a"); got != want {
		t.Fatalf("hash unstable: %s vs %s", got, want)
	}
	if len(HashRefreshToken("a")) != 64 {
		t.Fatalf("sha256 hex length = %d", len(HashRefreshToken("a")))
	}
}
