package polyline

import (
	"fmt"
	"math"
	"strings"
)

// Point is one decoded coordinate. Precision is 1e5, the same grid Goong
// Directions and this server use.
type Point struct {
	Lat float64
	Lng float64
}

// Decode reads one encoded polyline. An empty string is an empty path.
func Decode(encoded string) ([]Point, error) {
	points := make([]Point, 0)
	var lat, lng int64
	index := 0
	for index < len(encoded) {
		dLat, next, err := decodeValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = next
		dLng, next, err := decodeValue(encoded, index)
		if err != nil {
			return nil, err
		}
		index = next
		lat += dLat
		lng += dLng
		points = append(points, Point{Lat: float64(lat) / 1e5, Lng: float64(lng) / 1e5})
	}
	return points, nil
}

// Encode writes points on the 1e5 grid.
func Encode(points []Point) string {
	var lat, lng int64
	var out strings.Builder
	for _, point := range points {
		nextLat := int64(math.Round(point.Lat * 1e5))
		nextLng := int64(math.Round(point.Lng * 1e5))
		out.WriteString(encodeSigned(nextLat - lat))
		out.WriteString(encodeSigned(nextLng - lng))
		lat, lng = nextLat, nextLng
	}
	return out.String()
}

// Join stitches step polylines into one path. Each encoded string starts from
// (0, 0), so the strings themselves cannot be concatenated. The shared point
// where two steps meet is kept once. A chunk that does not decode fails the
// join so the caller can keep the overview line.
func Join(parts []string) (string, error) {
	var path []Point
	for _, part := range parts {
		if part == "" {
			continue
		}
		points, err := Decode(part)
		if err != nil {
			return "", err
		}
		if len(path) > 0 && len(points) > 0 && sameGrid(path[len(path)-1], points[0]) {
			points = points[1:]
		}
		path = append(path, points...)
	}
	return Encode(path), nil
}

func sameGrid(a, b Point) bool {
	return int64(math.Round(a.Lat*1e5)) == int64(math.Round(b.Lat*1e5)) &&
		int64(math.Round(a.Lng*1e5)) == int64(math.Round(b.Lng*1e5))
}

func decodeValue(encoded string, index int) (int64, int, error) {
	var result uint64
	var shift uint
	for {
		if index >= len(encoded) {
			return 0, index, fmt.Errorf("invalid encoded polyline")
		}
		bite := encoded[index] - 63
		index++
		result |= uint64(bite&0x1f) << shift
		shift += 5
		if bite < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		return -int64(result>>1) - 1, index, nil
	}
	return int64(result >> 1), index, nil
}

func encodeSigned(value int64) string {
	zigzag := uint64((value << 1) ^ (value >> 63))
	var out strings.Builder
	for zigzag >= 0x20 {
		out.WriteByte(byte((0x20 | (zigzag & 0x1f)) + 63))
		zigzag >>= 5
	}
	out.WriteByte(byte(zigzag + 63))
	return out.String()
}
