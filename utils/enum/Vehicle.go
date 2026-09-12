package enum

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:generate enumer -type=Vehicle

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
	case "", "bike", "motorbike", "motorcycle":
		return BIKE, nil
	case "car":
		return CAR, nil
	case "truck":
		return TRUCK, nil
	case "taxi":
		return TAXI, nil
	case "hd":
		return HD, nil
	default:
		return VehicleUnknown, fmt.Errorf("%w: %q (expected car, bike, taxi, truck, hd)", ErrUnsupportedVehicle, s)
	}
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
		return strings.ToLower(BIKE.String())
	}
	return strings.ToLower(n.String())
}

func (v Vehicle) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Goong())
}

func (v *Vehicle) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("Vehicle should be a string, got %s", data)
	}
	parsed, err := ParseVehicle(s)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

func (v Vehicle) MarshalText() ([]byte, error) {
	return []byte(v.Goong()), nil
}

func (v *Vehicle) UnmarshalText(text []byte) error {
	parsed, err := ParseVehicle(string(text))
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}

func (v Vehicle) Value() (driver.Value, error) {
	return v.Goong(), nil
}

func (v *Vehicle) Scan(value interface{}) error {
	if value == nil {
		*v = VehicleUnknown
		return nil
	}
	var str string
	switch raw := value.(type) {
	case []byte:
		str = string(raw)
	case string:
		str = raw
	case fmt.Stringer:
		str = raw.String()
	default:
		return fmt.Errorf("invalid value of Vehicle: %[1]T(%[1]v)", value)
	}
	parsed, err := ParseVehicle(str)
	if err != nil {
		return err
	}
	*v = parsed
	return nil
}
