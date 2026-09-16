package repository

import "testing"

func TestCooldownMinutesCaps(t *testing.T) {
	if got := cooldownMinutes(1); got != 20 {
		t.Fatalf("y=1 got %d", got)
	}
	if got := cooldownMinutes(9); got != 60 {
		t.Fatalf("y=9 got %d", got)
	}
	if got := cooldownMinutes(20); got != 60 {
		t.Fatalf("y=20 got %d", got)
	}
}
