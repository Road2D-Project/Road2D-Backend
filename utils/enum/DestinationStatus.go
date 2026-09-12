package enum

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// DestinationStatus is the DoAn1 lifecycle of a trip pin.
type DestinationStatus int

const (
	Editing DestinationStatus = iota
	Confirm
	Deleted
)

var destinationStatusName = map[DestinationStatus]string{
	Editing: "Editing",
	Confirm: "Confirm",
	Deleted: "Deleted",
}

func (status DestinationStatus) String() string {
	if name, ok := destinationStatusName[status]; ok {
		return name
	}
	return fmt.Sprintf("DestinationStatus(%d)", status)
}

func ParseDestinationStatus(str string) (DestinationStatus, error) {
	for value, name := range destinationStatusName {
		if name == str {
			return value, nil
		}
	}
	return Editing, fmt.Errorf("invalid DestinationStatus: %s", str)
}

func (status DestinationStatus) Value() (driver.Value, error) {
	return status.String(), nil
}

func (status *DestinationStatus) Scan(value interface{}) error {
	if value == nil {
		*status = Editing
		return nil
	}
	str, err := driverString(value)
	if err != nil {
		return fmt.Errorf("cannot scan type %T into DestinationStatus", value)
	}
	parsed, err := ParseDestinationStatus(str)
	if err != nil {
		return err
	}
	*status = parsed
	return nil
}

func (status DestinationStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(status.String())
}

func (status *DestinationStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := ParseDestinationStatus(str)
	if err != nil {
		return err
	}
	*status = parsed
	return nil
}
