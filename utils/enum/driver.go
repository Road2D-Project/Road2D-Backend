package enum

import "fmt"

func driverString(value interface{}) (string, error) {
	switch raw := value.(type) {
	case []byte:
		return string(raw), nil
	case string:
		return raw, nil
	default:
		return "", fmt.Errorf("unsupported driver value %T", value)
	}
}
