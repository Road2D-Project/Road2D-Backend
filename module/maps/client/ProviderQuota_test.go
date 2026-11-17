package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func testProviderQuota(t *testing.T, provider string, dailyLimit int) (*ProviderQuota, *miniredis.Miniredis) {
	t.Helper()
	t.Setenv(strings.ToUpper(provider)+"_DAILY_REQUEST_LIMIT", strconv.Itoa(dailyLimit))
	mini := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: mini.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	quota, err := NewProviderQuota(provider, redisClient)
	if err != nil {
		t.Fatal(err)
	}
	return quota, mini
}

func TestProviderQuotaSharesDailyBudgetAcrossClients(t *testing.T) {
	first, _ := testProviderQuota(t, "goong", 2)
	second, err := NewProviderQuota("goong", first.client)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := first.Allow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := second.Allow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := first.Allow(ctx); !errors.Is(err, ErrProviderQuotaExceeded) {
		t.Fatalf("third request returned %v, want exhausted daily budget", err)
	}
}

func TestProviderQuotaResetsAtUTCMidnight(t *testing.T) {
	quota, mini := testProviderQuota(t, "goong", 1)
	mini.SetTime(time.Date(2026, 11, 17, 23, 59, 59, 0, time.UTC))
	ctx := context.Background()
	if err := quota.Allow(ctx); err != nil {
		t.Fatal(err)
	}
	if err := quota.Allow(ctx); !errors.Is(err, ErrProviderQuotaExceeded) {
		t.Fatalf("second request returned %v", err)
	}
	mini.SetTime(time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC))
	if err := quota.Allow(ctx); err != nil {
		t.Fatalf("request after UTC midnight returned %v", err)
	}
}

func TestProviderQuotaConcurrentRequestsRespectDailyLimit(t *testing.T) {
	quota, _ := testProviderQuota(t, "goong", 3)
	var allowed atomic.Int64
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() {
			err := quota.Allow(context.Background())
			if err == nil {
				allowed.Add(1)
			} else if !errors.Is(err, ErrProviderQuotaExceeded) {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	if allowed.Load() != 3 {
		t.Fatalf("allowed %d requests, want 3", allowed.Load())
	}
}

func TestProviderQuotaRejectsInvalidDailyLimit(t *testing.T) {
	for _, value := range []string{"0", "-1", "unlimited"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("GOONG_DAILY_REQUEST_LIMIT", value)
			if _, err := NewProviderQuota("goong", nil); err == nil {
				t.Fatal("invalid daily limit was accepted")
			}
		})
	}
}

func TestGoongQuotaStopsOutgoingRequestsAcrossClients(t *testing.T) {
	quota, _ := testProviderQuota(t, "goong", 1)
	var calls atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	t.Cleanup(upstream.Close)
	t.Setenv("GOONG_MAP_CALC_API_KEY", "test-key")
	t.Setenv("GOONG_DOMAIN", upstream.URL)
	first := NewGoongClient(quota)
	second := NewGoongClient(quota)
	var out map[string]any
	if err := first.getJSON(context.Background(), "/v2/geocode", url.Values{}, &out); err != nil {
		t.Fatal(err)
	}
	if err := second.getJSON(context.Background(), "/v2/direction", url.Values{}, &out); !errors.Is(err, ErrProviderQuotaExceeded) {
		t.Fatalf("second outgoing request returned %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("provider received %d requests, want 1", calls.Load())
	}
}

func TestGoongQuotaFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	quota, _ := testProviderQuota(t, "goong", 1)
	t.Setenv("GOONG_MAP_CALC_API_KEY", "test-key")
	if err := quota.client.Close(); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err := NewGoongClient(quota).getJSON(context.Background(), "/v2/geocode", url.Values{}, &out)
	if !errors.Is(err, ErrQuotaUnavailable) {
		t.Fatalf("request returned %v, want unavailable quota", err)
	}
}
