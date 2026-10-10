package geo

import (
	"Road-To-Destination-BE/module/tracking/model"
	"math"
)

const earthRadius = 6371000.0

// toXY: đổi p sang mét, gốc toạ độ tại ref
func toXY(ref, p model.Point) (x, y float64) {
	const d2r = math.Pi / 180
	x = (p.Lng - ref.Lng) * d2r * earthRadius * math.Cos(ref.Lat*d2r)
	y = (p.Lat - ref.Lat) * d2r * earthRadius
	return
}

// PointToSegment returns the distance in meters from c to segment ab.
// t is 0 at a and 1 at b. segLen is |AB| in meters.
func PointToSegment(a, b, c model.Point) (dist, t, segLen float64) {
	abX, abY := toXY(a, b)
	acX, acY := toXY(a, c)

	len2 := abX*abX + abY*abY
	if len2 == 0 {
		dist = math.Hypot(acX, acY)
		t = 0
		segLen = 0
		return
	}

	t = math.Max(0, math.Min(1, (acX*abX+acY*abY)/len2))
	dist = math.Hypot(acX-t*abX, acY-t*abY)
	segLen = math.Sqrt(len2)
	return
}
