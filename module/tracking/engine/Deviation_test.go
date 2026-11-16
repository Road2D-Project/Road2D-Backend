package engine

import (
	"testing"

	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// 0.001 deg of latitude is about 111 m, well past geo.OffRouteM.
const offRouteLat = 0.001

func TestNewTravelProgressStartsWithThreeLives(t *testing.T) {
	prg := NewTravelProgress(uuid.New(), uuid.New())
	if prg.Lives != 3 {
		t.Fatalf("lives %d", prg.Lives)
	}
}

func TestOffRoutePointKeepsTheBranchCursor(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	c := model.Point{Lat: 10.79, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{
		{Points: []model.Point{a, b}, Ready: true},
		{Points: []model.Point{b, c}, Ready: true},
	}}

	mid := model.Point{Lat: 10.775, Lng: 106.70}
	onRoute, ok := snapPointOnBranch(hopCursor{}, branch, mid)
	if !ok || onRoute.offRoute || onRoute.dist > 1 || onRoute.progress == 0 {
		t.Fatalf("on-route sample %+v ok=%v", onRoute, ok)
	}

	// Far to the side of the polyline, level with the end of the hop. Without
	// the lock the cursor would step onto hop 1.
	beside := model.Point{Lat: b.Lat, Lng: b.Lng + offRouteLat}
	off, ok := snapPointOnBranch(onRoute, branch, beside)
	if !ok || !off.offRoute {
		t.Fatalf("off-route sample %+v ok=%v", off, ok)
	}
	if off.dist <= geo.OffRouteM {
		t.Fatalf("dist %.1f should exceed %.0f", off.dist, geo.OffRouteM)
	}
	if off.hop != onRoute.hop || off.seg != onRoute.seg || off.progress != onRoute.progress {
		t.Fatalf("cursor moved while off route: %+v -> %+v", onRoute, off)
	}

	back, ok := snapPointOnBranch(off, branch, b)
	if !ok || back.offRoute || back.hop != 1 {
		t.Fatalf("return sample %+v ok=%v", back, ok)
	}
}

func TestOffRouteSampleNeverFinishesTheBranch(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{{Points: []model.Point{a, b}, Ready: true}}}

	start, _ := snapPointOnBranch(hopCursor{}, branch, a)
	beside := model.Point{Lat: b.Lat, Lng: b.Lng + offRouteLat}
	off, ok := snapPointOnBranch(start, branch, beside)
	if !ok || !off.offRoute || off.finished {
		t.Fatalf("off-route sample %+v", off)
	}
	done, _ := snapPointOnBranch(off, branch, b)
	if !done.finished {
		t.Fatalf("on-route end sample %+v", done)
	}
}

func TestSnapPointOnRedirectUsesItsOwnCursor(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{{Points: []model.Point{a, b}, Ready: true}}}

	side := model.Point{Lat: 10.77, Lng: 106.71}
	route := geo.NewRoute([]model.Point{side, {Lat: 10.78, Lng: 106.71}, b})

	cursor, _ := snapPointOnBranch(hopCursor{}, branch, a)
	cursor = snapPointOnRedirect(cursor, route, 7, side)
	if cursor.redirectGen != 7 || cursor.redirectSeg != 0 || cursor.redirectArrived {
		t.Fatalf("start of redirect %+v", cursor)
	}
	if cursor.hop != 0 || cursor.seg != 0 || cursor.route == nil {
		t.Fatalf("branch cursor changed %+v", cursor)
	}

	cursor = snapPointOnRedirect(cursor, route, 8, model.Point{Lat: 10.78, Lng: 106.71})
	if cursor.redirectGen != 8 || cursor.redirectSeg == 0 && cursor.redirectProgress == 0 {
		t.Fatalf("mid redirect %+v", cursor)
	}

	cursor = snapPointOnRedirect(cursor, route, 8, b)
	if !cursor.redirectArrived {
		t.Fatalf("end of redirect %+v", cursor)
	}

	cursor = snapPointOnRedirect(cursor, nil, 0, b)
	if cursor.redirectRoute != nil || cursor.redirectArrived || cursor.redirectGen != 0 {
		t.Fatalf("cleared redirect %+v", cursor)
	}
}

func TestSnapPointOnRedirectResetsOnNewRoute(t *testing.T) {
	first := geo.NewRoute([]model.Point{{Lat: 10.77, Lng: 106.70}, {Lat: 10.78, Lng: 106.70}})
	second := geo.NewRoute([]model.Point{{Lat: 10.78, Lng: 106.70}, {Lat: 10.79, Lng: 106.70}})

	cursor := snapPointOnRedirect(hopCursor{}, first, 1, model.Point{Lat: 10.78, Lng: 106.70})
	if !cursor.redirectArrived {
		t.Fatalf("first route %+v", cursor)
	}
	cursor = snapPointOnRedirect(cursor, second, 2, model.Point{Lat: 10.78, Lng: 106.70})
	if cursor.redirectArrived || cursor.redirectSeg != 0 || cursor.redirectGen != 2 {
		t.Fatalf("second route %+v", cursor)
	}
}

func TestApplySnapIgnoresRedirectFieldsFromAnOldGeneration(t *testing.T) {
	prg := NewTravelProgress(uuid.New(), uuid.New())
	prg.DriveStatus = enum.DEVIATED
	prg.Redirect = &RedirectState{Generation: 3}
	prg.updates = make(chan SnapUpdate, 1)

	prg.updates <- SnapUpdate{RedirectGen: 2, RedirectArrived: true, RedirectDistM: 5, OffRoute: true, DistM: 80}
	prg.ApplySnap()
	if prg.RedirectArrived || prg.RedirectDistM != 0 {
		t.Fatalf("stale generation applied %+v", prg)
	}
	if !prg.OffRoute || prg.LastDistM != 80 {
		t.Fatalf("branch fields were not copied %+v", prg)
	}

	prg.updates <- SnapUpdate{RedirectGen: 3, RedirectArrived: true, RedirectDistM: 5}
	prg.ApplySnap()
	if !prg.RedirectArrived || prg.RedirectDistM != 5 {
		t.Fatalf("current generation ignored %+v", prg)
	}
}

func TestRedirectResultBufferKeepsTheNewest(t *testing.T) {
	prg := NewTravelProgress(uuid.New(), uuid.New())
	for gen := uint64(1); gen <= 5; gen++ {
		prg.OfferRedirectResult(RedirectResult{Generation: gen})
	}
	got := prg.DrainRedirectResults()
	if len(got) != redirectResultBuffer || got[len(got)-1].Generation != 5 {
		t.Fatalf("drained %+v", got)
	}
	if again := prg.DrainRedirectResults(); len(again) != 0 {
		t.Fatalf("second drain %+v", again)
	}
}

func TestClearRedirectBumpsTheGeneration(t *testing.T) {
	prg := NewTravelProgress(uuid.New(), uuid.New())
	gen := prg.NextRedirectGeneration()
	prg.Redirect = &RedirectState{Generation: gen}
	prg.RedirectArrived = true
	prg.ClearRedirect()
	if prg.Redirect != nil || prg.RedirectArrived || prg.RedirectGeneration == gen {
		t.Fatalf("after clear %+v", prg)
	}
}
