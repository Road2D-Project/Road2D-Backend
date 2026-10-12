package model

import (
	"Road-To-Destination-BE/utils/polyline"

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
	raw, err := polyline.Decode(encoded)
	if err != nil {
		return nil, err
	}
	points := make([]Point, len(raw))
	for i, point := range raw {
		points[i] = Point{Lat: point.Lat, Lng: point.Lng}
	}
	return points, nil
}
