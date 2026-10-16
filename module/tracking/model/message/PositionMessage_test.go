package message

import "testing"

func TestSignalBars(t *testing.T) {
	cases := []struct {
		dbm  int
		bars int
	}{
		{dbm: -70, bars: 5},
		{dbm: -69, bars: 5},
		{dbm: -85, bars: 4},
		{dbm: -100, bars: 3},
		{dbm: -110, bars: 2},
		{dbm: -120, bars: 1},
		{dbm: -121, bars: 0},
	}
	for _, tc := range cases {
		got := PositionMessage{SignalDbm: tc.dbm}.SignalBars()
		if got != tc.bars {
			t.Fatalf("dbm %d bars %d, want %d", tc.dbm, got, tc.bars)
		}
	}
}

func TestBatteryClampsToAPercent(t *testing.T) {
	if got := (PositionMessage{BatteryPercent: -4}).Battery(); got != 0 {
		t.Fatalf("low %d", got)
	}
	if got := (PositionMessage{BatteryPercent: 140}).Battery(); got != 100 {
		t.Fatalf("high %d", got)
	}
	if got := (PositionMessage{BatteryPercent: 42}).Battery(); got != 42 {
		t.Fatalf("mid %d", got)
	}
}
