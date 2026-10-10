package geo

import (
	"Road-To-Destination-BE/module/tracking/model"
	"fmt"
	"math"
)

const (
	segBack   = 2
	segAhead  = 10
	offRouteM = 50.0
	arriveM   = 30.0
)

type Route struct {
	HistoryPoints []model.Point
	Points        []model.Point
	Cumulation    []float64
}
type SnapResult struct {
	Seg            int
	Progress, Dist float64
}

func (s *SnapResult) String() string {
	return fmt.Sprintf("seg: %d, progress: %f, dist: %f", s.Seg, s.Progress, s.Dist)
}

func NewRoute(points []model.Point) *Route {
	cumulation := make([]float64, len(points))
	for i := 1; i < len(points); i++ {
		x, y := toXY(points[i-1], points[i])
		cumulation[i] = math.Hypot(x, y) + cumulation[i-1]
	}
	// new route have no idea about the previous one
	// need to shift
	return &Route{
		Points:     points,
		Cumulation: cumulation,
	}
}

// TotalM is the length of the polyline in meters.
func (r *Route) TotalM() float64 {
	if r == nil || len(r.Cumulation) == 0 {
		return 0
	}
	return r.Cumulation[len(r.Cumulation)-1]
}

// Shift adds base meters to every cumulation so the next hop continues the previous one.
func (r *Route) Shift(base float64) {
	for i := range r.Cumulation {
		r.Cumulation[i] += base
	}
}

// AtEnd reports that the snap is on the last segment and within arriveM of its end.
func (r *Route) AtEnd(seg int, progress, dist float64) bool {
	nSeg := len(r.Points) - 1
	if r == nil || nSeg < 1 || dist > offRouteM || seg != nSeg-1 {
		return false
	}
	return progress >= r.TotalM()-arriveM
}

// Snap searches a window around lastSeg. Segment i joins Points[i] and Points[i+1].
// A miss farther than offRouteM searches the whole polyline.
// An unusable polyline returns +Inf so the caller does not treat it as on the road.
func (r *Route) Snap(c model.Point, lastSeg int) SnapResult {
	nSeg := len(r.Points) - 1
	if r == nil || nSeg < 1 {
		return SnapResult{Dist: math.Inf(1)}
	}
	if lastSeg < 0 {
		lastSeg = 0
	}
	if lastSeg >= nSeg {
		lastSeg = nSeg - 1
	}
	low, high := max(0, lastSeg-segBack), min(nSeg-1, lastSeg+segAhead)
	best := r.Scan(c, low, high)
	if best.Dist > offRouteM {
		best = r.Scan(c, 0, nSeg-1)
	}
	return best
}

func (r *Route) Scan(c model.Point, low, high int) SnapResult {
	best := SnapResult{Dist: math.Inf(1)}
	if low > high {
		return best
	}
	for i := low; i <= high; i++ {
		d, t, l := PointToSegment(r.Points[i], r.Points[i+1], c)
		if d < best.Dist {
			best = SnapResult{
				Seg:      i,
				Progress: r.Cumulation[i] + t*l,
				Dist:     d,
			}
		}
	}
	return best
}
