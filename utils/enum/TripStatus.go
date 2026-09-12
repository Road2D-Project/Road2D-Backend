package enum

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type TripStatus int

const (
	TripPlanning TripStatus = iota
	TripLocked
)

var tripStatusName = map[TripStatus]string{
	TripPlanning: "planning",
	TripLocked:   "locked",
}

func (status TripStatus) String() string {
	if name, ok := tripStatusName[status]; ok {
		return name
	}
	return fmt.Sprintf("TripStatus(%d)", status)
}

func ParseTripStatus(str string) (TripStatus, error) {
	for value, name := range tripStatusName {
		if name == str {
			return value, nil
		}
	}
	return TripPlanning, fmt.Errorf("invalid TripStatus: %s", str)
}

func (status TripStatus) Value() (driver.Value, error) {
	return status.String(), nil
}

func (status *TripStatus) Scan(value interface{}) error {
	if value == nil {
		*status = TripPlanning
		return nil
	}
	str, err := driverString(value)
	if err != nil {
		return fmt.Errorf("cannot scan type %T into TripStatus", value)
	}
	parsed, err := ParseTripStatus(str)
	if err != nil {
		return err
	}
	*status = parsed
	return nil
}

func (status TripStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(status.String())
}

func (status *TripStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := ParseTripStatus(str)
	if err != nil {
		return err
	}
	*status = parsed
	return nil
}
