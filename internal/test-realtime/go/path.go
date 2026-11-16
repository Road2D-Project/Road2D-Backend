package main

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Point is one GPS sample. The grid is 1e5, the same one the server uses for Goong polylines.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// Route is the polyline the phone walks. Length is meters along the vertices.
type Route struct {
	Points []Point
	CumM   []float64
	Length float64
	Source string
}

const earthRadiusM = 6371000.0

// Decode reads a Google/Goong encoded polyline (precision 1e5).
func Decode(encoded string) ([]Point, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("polyline rỗng")
	}
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

func decodeValue(encoded string, index int) (int64, int, error) {
	var result uint64
	var shift uint
	for {
		if index >= len(encoded) {
			return 0, index, errors.New("polyline không hợp lệ")
		}
		bite := encoded[index] - 63
		index++
		result |= uint64(bite&0x1f) << shift
		shift += 5
		if bite < 0x20 {
			break
		}
		if shift > 60 {
			return 0, index, errors.New("polyline không hợp lệ")
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

// BuildRoute drops vertices closer than half a meter and measures the walk.
func BuildRoute(points []Point, source string) (Route, error) {
	if len(points) == 0 {
		return Route{}, errors.New("đường không có điểm")
	}
	cleaned := make([]Point, 0, len(points))
	for _, point := range points {
		if !validCoord(point) {
			return Route{}, fmt.Errorf("tọa độ ngoài phạm vi: lat %.6f lng %.6f", point.Lat, point.Lng)
		}
		if len(cleaned) > 0 && metersBetween(cleaned[len(cleaned)-1], point) < 0.5 {
			continue
		}
		cleaned = append(cleaned, point)
	}
	if len(cleaned) < 2 {
		return Route{}, errors.New("đường cần ít nhất 2 điểm khác nhau")
	}
	cum := make([]float64, len(cleaned))
	for i := 1; i < len(cleaned); i++ {
		cum[i] = cum[i-1] + metersBetween(cleaned[i-1], cleaned[i])
	}
	if cum[len(cum)-1] <= 0 {
		return Route{}, errors.New("polyline không có chiều dài")
	}
	return Route{Points: cleaned, CumM: cum, Length: cum[len(cum)-1], Source: source}, nil
}

func validCoord(p Point) bool {
	return p.Lat >= -90 && p.Lat <= 90 && p.Lng >= -180 && p.Lng <= 180
}

func metersBetween(a, b Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := lat2 - lat1
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	if h > 1 {
		h = 1
	}
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}

// bearing is radians clockwise from north.
func bearing(a, b Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180
	y := math.Sin(dLng) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLng)
	return math.Atan2(y, x)
}

// at returns the point at dist meters and the bearing of the segment it sits on.
func (r Route) at(dist float64) (Point, float64) {
	n := len(r.Points)
	if dist <= 0 {
		return r.Points[0], bearing(r.Points[0], r.Points[1])
	}
	if dist >= r.Length {
		return r.Points[n-1], bearing(r.Points[n-2], r.Points[n-1])
	}
	i := 1
	for i < n-1 && r.CumM[i] < dist {
		i++
	}
	span := r.CumM[i] - r.CumM[i-1]
	t := 0.0
	if span > 0 {
		t = (dist - r.CumM[i-1]) / span
	}
	point := Point{
		Lat: r.Points[i-1].Lat + (r.Points[i].Lat-r.Points[i-1].Lat)*t,
		Lng: r.Points[i-1].Lng + (r.Points[i].Lng-r.Points[i-1].Lng)*t,
	}
	return point, bearing(r.Points[i-1], r.Points[i])
}

// offsetMeters shifts the point to the right of travel. Negative meters go left.
func offsetMeters(p Point, bearingRad, meters float64) Point {
	turn := bearingRad + math.Pi/2
	north := meters * math.Cos(turn)
	east := meters * math.Sin(turn)
	return Point{
		Lat: p.Lat + north/111320.0,
		Lng: p.Lng + east/(111320.0*math.Cos(p.Lat*math.Pi/180)),
	}
}
