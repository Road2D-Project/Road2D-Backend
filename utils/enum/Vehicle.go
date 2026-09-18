package enum

import (
	"errors"
	"fmt"
	"strings"
)

//go:generate go tool enumer -type=Vehicle -json -text -sql -transform=lower

type Vehicle uint8

const (
	VehicleUnknown Vehicle = iota
	CAR
	BIKE
	TRUCK
	TAXI
	HD
)

var ErrUnsupportedVehicle = errors.New("unsupported vehicle")

func ParseVehicle(s string) (Vehicle, error) {
	key := strings.ToLower(strings.TrimSpace(s))
	switch key {
	case "", "motorbike", "motorcycle":
		return BIKE, nil
	}
	parsed, err := VehicleString(key)
	if err != nil || parsed == VehicleUnknown {
		return VehicleUnknown, fmt.Errorf("%w: %q (expected car, bike, taxi, truck, hd)", ErrUnsupportedVehicle, s)
	}
	return parsed, nil
}

func (v *Vehicle) UnmarshalParam(param string) error {
	parsed, err := ParseVehicle(param)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

func (v Vehicle) Normalized() (Vehicle, error) {
	return v.NormalizedOr(BIKE)
}

func (v Vehicle) NormalizedOr(fallback Vehicle) (Vehicle, error) {
	if v == VehicleUnknown {
		if fallback == VehicleUnknown || !fallback.IsAVehicle() {
			return BIKE, nil
		}
		return fallback, nil
	}
	if !v.IsAVehicle() {
		return VehicleUnknown, fmt.Errorf("%w: %d", ErrUnsupportedVehicle, v)
	}
	return v, nil
}

func (v Vehicle) Goong() string {
	n, err := v.Normalized()
	if err != nil {
		return BIKE.String()
	}
	return n.String()
}
