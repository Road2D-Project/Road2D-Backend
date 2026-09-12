package service

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidLatLng = errors.New("invalid lat,lng")

func parsePoint(s string) (float64, float64, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidLatLng, s)
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidLatLng, s)
	}
	lng, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: %q", ErrInvalidLatLng, s)
	}
	return lat, lng, nil
}

func firstPoint(s string) (float64, float64, error) {
	chunks := strings.Split(s, ";")
	return parsePoint(chunks[0])
}

func lastPoint(s string) (float64, float64, error) {
	chunks := strings.Split(s, ";")
	return parsePoint(chunks[len(chunks)-1])
}
