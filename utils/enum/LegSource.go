package enum

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type LegSource int

const (
	LegSourceDirection LegSource = iota
	LegSourceTrip
)

var legSourceName = map[LegSource]string{
	LegSourceDirection: "direction",
	LegSourceTrip:      "trip",
}

func (source LegSource) String() string {
	if name, ok := legSourceName[source]; ok {
		return name
	}
	return fmt.Sprintf("LegSource(%d)", source)
}

func ParseLegSource(str string) (LegSource, error) {
	for value, name := range legSourceName {
		if name == str {
			return value, nil
		}
	}
	return LegSourceDirection, fmt.Errorf("invalid LegSource: %s", str)
}

func (source LegSource) Value() (driver.Value, error) {
	return source.String(), nil
}

func (source *LegSource) Scan(value interface{}) error {
	if value == nil {
		*source = LegSourceDirection
		return nil
	}
	str, err := driverString(value)
	if err != nil {
		return fmt.Errorf("cannot scan type %T into LegSource", value)
	}
	parsed, err := ParseLegSource(str)
	if err != nil {
		return err
	}
	*source = parsed
	return nil
}

func (source LegSource) MarshalJSON() ([]byte, error) {
	return json.Marshal(source.String())
}

func (source *LegSource) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := ParseLegSource(str)
	if err != nil {
		return err
	}
	*source = parsed
	return nil
}
