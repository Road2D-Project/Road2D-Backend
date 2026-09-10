package enum

import (
	"errors"
	"testing"
)

func TestParseVehicleAliases(t *testing.T) {
	cases := map[string]Vehicle{
		"":           BIKE,
		"bike":       BIKE,
		"BIKE":       BIKE,
		"motorbike":  BIKE,
		"motorcycle": BIKE,
		"car":        CAR,
		"taxi":       TAXI,
		"truck":      TRUCK,
		"hd":         HD,
	}
	for in, want := range cases {
		got, err := ParseVehicle(in)
		if err != nil {
			t.Fatalf("ParseVehicle(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("ParseVehicle(%q)=%s want %s", in, got, want)
		}
	}
}

func TestParseVehicleRejectsUnknown(t *testing.T) {
	_, err := ParseVehicle("walk")
	if !errors.Is(err, ErrUnsupportedVehicle) {
		t.Fatalf("got %v", err)
	}
}

func TestVehicleGoongLowercase(t *testing.T) {
	if CAR.Goong() != "car" || BIKE.Goong() != "bike" {
		t.Fatalf("goong values: car=%s bike=%s", CAR.Goong(), BIKE.Goong())
	}
}

func TestVehicleNormalizedOr(t *testing.T) {
	got, err := VehicleUnknown.NormalizedOr(CAR)
	if err != nil || got != CAR {
		t.Fatalf("trip default: %s %v", got, err)
	}
	got, err = VehicleUnknown.NormalizedOr(BIKE)
	if err != nil || got != BIKE {
		t.Fatalf("direction default: %s %v", got, err)
	}
}
