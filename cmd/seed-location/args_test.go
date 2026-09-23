package main

import (
	"reflect"
	"testing"
)

func TestRepairSeedArgsPowerShellDecimals(t *testing.T) {
	got := RepairSeedArgs([]string{"--", "-lat=10", ".7486", "-lng=106", ".6601"})
	want := []string{"-lat=10.7486", "-lng=106.6601"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRepairSeedArgsEmptyEquals(t *testing.T) {
	got := RepairSeedArgs([]string{"--", "-coords=", " 10.790060025728994,106.78967714926611"})
	want := []string{"-coords=10.790060025728994,106.78967714926611"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRepairSeedArgsBareNumbers(t *testing.T) {
	got := RepairSeedArgs([]string{"10", ".7486", "106", ".6601"})
	want := []string{"10.7486", "106.6601"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
