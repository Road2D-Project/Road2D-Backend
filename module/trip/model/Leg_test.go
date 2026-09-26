package model

import (
	"testing"
	"time"
)

const dayTTL = 24 * time.Hour

func TestLegExpiredUsesFallbackWhenTTLUnset(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	leg := &Leg{LastComputedAt: now.Add(-25 * time.Hour)}
	if !leg.Expired(now, dayTTL) {
		t.Fatal("a day-old leg is stale under a one day fallback")
	}
	leg.LastComputedAt = now.Add(-time.Hour)
	if leg.Expired(now, dayTTL) {
		t.Fatal("an hour-old leg is still fresh")
	}
}

func TestLegExpiredPrefersOwnTTL(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	leg := &Leg{LastComputedAt: now.Add(-2 * time.Hour), TTLSeconds: 3600}
	if !leg.Expired(now, dayTTL) {
		t.Fatal("the leg's own one hour TTL must win over the fallback")
	}
}

func TestLegExpiredAtTheBoundary(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	leg := &Leg{LastComputedAt: now.Add(-dayTTL)}
	if leg.Expired(now, dayTTL) {
		t.Fatal("expiry is exclusive: the leg lives until the window has passed")
	}
	leg.LastComputedAt = now.Add(-dayTTL - time.Nanosecond)
	if !leg.Expired(now, dayTTL) {
		t.Fatal("one tick past the window is stale")
	}
}

func TestNilLegIsExpired(t *testing.T) {
	var leg *Leg
	if !leg.Expired(time.Now(), dayTTL) {
		t.Fatal("a missing leg can never be reused")
	}
}
