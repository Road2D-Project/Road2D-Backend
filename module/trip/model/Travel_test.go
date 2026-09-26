package model

import (
	"testing"
	"time"
)

func TestFrozenTravelNeverExpires(t *testing.T) {
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	travel := &Travel{IsFrozen: true, LastComputedAt: now.Add(-365 * dayTTL)}
	if travel.Expired(now, dayTTL) {
		t.Fatal("frozen travel is shared history and stays valid")
	}
	travel.IsFrozen = false
	if !travel.Expired(now, dayTTL) {
		t.Fatal("an unfrozen year-old travel is stale")
	}
}

func TestNilTravelIsExpired(t *testing.T) {
	var travel *Travel
	if !travel.Expired(time.Now(), dayTTL) {
		t.Fatal("a missing travel can never be reused")
	}
}
