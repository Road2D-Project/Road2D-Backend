package model

import (
	"fmt"

	"github.com/google/uuid"
)

type Hop struct {
	FromDestinationID uuid.UUID
	ToDestinationID   uuid.UUID
	Points            []Point // decode một lần lúc load
	DistanceM         float64
	DurationS         float64
	Ready             bool // false khi thiếu travel hoặc polyline rỗng
}

func DecodePolyline(encoded string) ([]Point, error) {
	var points []Point

	var lat, lng int64
	var index int

	for index < len(encoded) {
		dLat, nextIndex, err := decodeValue(encoded, index)
		if err != nil {
			return nil, err
		}

		index = nextIndex

		dLng, nextIndex, err := decodeValue(encoded, index)
		if err != nil {
			return nil, err
		}

		index = nextIndex

		lat += dLat
		lng += dLng

		points = append(points, Point{
			Lat: float64(lat) / 1e5,
			Lng: float64(lng) / 1e5,
		})
	}

	return points, nil
}
func decodeValue(encoded string, index int) (int64, int, error) {
	var result uint64
	var shift uint

	for {
		if index >= len(encoded) {
			return 0, index, fmt.Errorf("invalid encoded polyline")
		}

		b := encoded[index] - 63
		index++

		result |= uint64(b&0x1f) << shift
		shift += 5

		if b < 0x20 {
			break
		}
	}

	var value int64

	if result&1 != 0 {
		value = -int64(result>>1) - 1
	} else {
		value = int64(result >> 1)
	}

	return value, index, nil
}
