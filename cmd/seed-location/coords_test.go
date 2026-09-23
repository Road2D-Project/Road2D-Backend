package main

import "testing"

func TestParseSeedCoordsPairs(t *testing.T) {
	got, err := parseSeedCoords("10.7725,106.6980; 10.8721512, 106.803008")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Lat != 10.7725 || got[0].Lng != 106.6980 {
		t.Fatalf("first=%+v", got[0])
	}
	if got[1].Lat != 10.8721512 || got[1].Lng != 106.803008 {
		t.Fatalf("second=%+v", got[1])
	}
}

func TestParseSeedCoordsEmptyIsNil(t *testing.T) {
	got, err := parseSeedCoords("  ")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("got %+v", got)
	}
}

func TestParseSeedCoordTokensPowerShellSplit(t *testing.T) {
	got, err := parseSeedCoordTokens("10.7486", "106.6601")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Lat != 10.7486 || got[0].Lng != 106.6601 {
		t.Fatalf("got %+v", got)
	}
	got, err = parseSeedCoordTokens("", "10.794340230004332,106.78941165237038")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Lat != 10.794340230004332 {
		t.Fatalf("got %+v", got)
	}
}

func TestParseSeedCoordsRejectsBadPair(t *testing.T) {
	if _, err := parseSeedCoords("10.77"); err == nil {
		t.Fatal("expected error for missing lng")
	}
	if _, err := parseSeedCoords("91,0"); err == nil {
		t.Fatal("expected range error")
	}
}
