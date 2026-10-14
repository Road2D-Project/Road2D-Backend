package engine

import (
	"testing"
	"time"

	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func TestSnapPointOnBranchStepsOntoTheNextHop(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	c := model.Point{Lat: 10.79, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{
		{Points: []model.Point{a, b}, Ready: true},
		{Points: []model.Point{b, c}, Ready: true},
	}}

	prg := &TravelProgress{}
	stayed, ok := prg.snapPointOnBranch(hopCursor{}, branch, a)
	if !ok || stayed.hop != 0 || stayed.route == nil {
		t.Fatalf("start of hop: %+v ok=%v", stayed, ok)
	}
	stepped, ok := prg.snapPointOnBranch(stayed, branch, b)
	if !ok || stepped.hop != 1 {
		t.Fatalf("end of hop: %+v ok=%v", stepped, ok)
	}
}

func TestApplySnapCopiesWorkerResult(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{{
		Points: []model.Point{a, b},
		Ready:  true,
	}}}
	prg := NewTravelProgress(uuid.New(), uuid.New())
	prg.Lat = a.Lat
	prg.Lng = a.Lng
	prg.ObservedAt = time.Now()
	prg.StartSnap(branch)
	defer func() {
		prg.StopSnap()
		waitSnapWorkers(t, 0)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		prg.SubmitSample()
		time.Sleep(5 * time.Millisecond)
		prg.ApplySnap()
		if prg.Route != nil && prg.HopIndex == 0 {
			return
		}
	}
	t.Fatal("room goroutine never received a snap update")
}

func TestRestartSnapDoesNotKeepOldWorkers(t *testing.T) {
	a := model.Point{Lat: 10.77, Lng: 106.70}
	b := model.Point{Lat: 10.78, Lng: 106.70}
	branch := &model.TravelBranch{Hops: []model.Hop{{
		Points: []model.Point{a, b},
		Ready:  true,
	}}}
	prg := NewTravelProgress(uuid.New(), uuid.New())
	prg.DriveStatus = enum.OnROUTE
	for range 30 {
		prg.StartSnap(branch)
	}
	waitSnapWorkers(t, 1)
	prg.StopSnap()
	waitSnapWorkers(t, 0)
}

func waitSnapWorkers(t *testing.T, want int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if activeSnapWorkers.Load() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("snap workers = %d, want %d", activeSnapWorkers.Load(), want)
}
