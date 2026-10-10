package geo

import (
	"math"
	"testing"

	"Road-To-Destination-BE/module/tracking/model"
)

func TestPointToSegmentEndsAreOnTheSegment(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}

	distA, tA, segLen := PointToSegment(a, b, a)
	distB, tB, _ := PointToSegment(a, b, b)
	if segLen < 1000 || segLen > 1200 {
		t.Fatalf("segment length %v m", segLen)
	}
	if distA > 1 || tA > 0.01 {
		t.Fatalf("at A dist %v t %v", distA, tA)
	}
	if distB > 1 || tB < 0.99 {
		t.Fatalf("at B dist %v t %v", distB, tB)
	}
}

func TestPointToSegmentEastOffsetIsMeters(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	const offsetM = 80.0
	dLng := offsetM / (earthRadius * math.Cos(a.Lat*math.Pi/180) * math.Pi / 180)
	c := model.Point{Lat: a.Lat, Lng: a.Lng + dLng}

	dist, _, _ := PointToSegment(a, b, c)
	if math.Abs(dist-offsetM) > 1 {
		t.Fatalf("dist %v m, want about %v", dist, offsetM)
	}
}

func TestSnapUsesTheOnlySegmentAndKeepsProgress(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	route := NewRoute([]model.Point{a, b})

	atB := route.Snap(b, 0)
	if atB.Seg != 0 || atB.Dist > 1 {
		t.Fatalf("snap at B %+v", atB)
	}
	if math.Abs(atB.Progress-route.TotalM()) > 1 {
		t.Fatalf("progress %v total %v", atB.Progress, route.TotalM())
	}
	if !route.AtEnd(atB.Seg, atB.Progress, atB.Dist) {
		t.Fatal("standing on B should finish the hop")
	}

	empty := NewRoute(nil)
	if empty.Snap(a, 0).Dist != math.Inf(1) {
		t.Fatal("empty polyline should not look on-route")
	}
}

func TestShiftContinuesCumulation(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	first := NewRoute([]model.Point{a, b})
	next := NewRoute([]model.Point{b, a})
	base := first.TotalM()
	next.Shift(base)
	if math.Abs(next.Cumulation[0]-base) > 0.01 {
		t.Fatalf("cumulation[0] %v", next.Cumulation[0])
	}
	if math.Abs(next.TotalM()-(base+first.TotalM())) > 1 {
		t.Fatalf("total %v want %v", next.TotalM(), base+first.TotalM())
	}
}
