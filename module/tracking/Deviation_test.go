package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"
	"Road-To-Destination-BE/utils/polyline"

	"github.com/google/uuid"
)

var errRouterDown = errors.New("router down")

// stubRouter answers every call with the same route or error and counts calls.
type stubRouter struct {
	calls    atomic.Int32
	polyline string
	err      error
}

func (r *stubRouter) Redirect(context.Context, float64, float64, float64, float64) (string, float64, float64, error) {
	r.calls.Add(1)
	if r.err != nil {
		return "", 0, 0, r.err
	}
	return r.polyline, 120, 40, nil
}

var (
	stopA = model.Point{Lat: 10.77, Lng: 106.70}
	stopB = model.Point{Lat: 10.78, Lng: 106.70}
	stopC = model.Point{Lat: 10.79, Lng: 106.70}
	// About 110 m east of the polyline.
	beside = model.Point{Lat: 10.775, Lng: 106.701}
)

func deviationFixture(router RedirectRouter) (*TrackingCoordinator, *recordSession, *engine.TravelProgress) {
	rider := uuid.New()
	branchID := uuid.New()
	branch := &model.TravelBranch{
		ID: branchID,
		Stops: []model.Stop{
			{DestinationID: uuid.New(), Name: "A", Latitude: stopA.Lat, Longitude: stopA.Lng},
			{DestinationID: uuid.New(), Name: "B", Latitude: stopB.Lat, Longitude: stopB.Lng},
			{DestinationID: uuid.New(), Name: "C", Latitude: stopC.Lat, Longitude: stopC.Lng},
		},
		Hops: []model.Hop{
			{Points: []model.Point{stopA, stopB}, Ready: true},
			{Points: []model.Point{stopB, stopC}, Ready: true},
		},
	}
	progress := engine.NewTravelProgress(rider, branchID)
	progress.DriveStatus = enum.OnROUTE
	progress.Lat = stopA.Lat
	progress.Lng = stopA.Lng
	coord := &TrackingCoordinator{
		router: router,
		tripInformation: &model.TripInformation{
			Status:   enum.TripActive,
			Riders:   map[uuid.UUID]*model.Rider{rider: {UserID: rider, Role: enum.TripRoleLeader}},
			Branches: map[uuid.UUID]*model.TravelBranch{branchID: branch},
		},
		progresses: map[uuid.UUID]*engine.TravelProgress{rider: progress},
	}
	return coord, &recordSession{}, progress
}

// feed stands in for ApplySnap: it records what the worker would have reported
// for one sample and runs the state machine once.
func feed(coord *TrackingCoordinator, s *recordSession, p *engine.TravelProgress, at time.Time, point model.Point, offRoute bool) {
	p.Lat = point.Lat
	p.Lng = point.Lng
	p.LastSnapAt = at
	p.OffRoute = offRoute
	p.LastDistM = 0
	if offRoute {
		p.LastDistM = 110
	}
	coord.applyDeviation(s, p, at)
}

func msgTypes(s *recordSession) []string {
	out := make([]string, len(s.broadcasts))
	for i, b := range s.broadcasts {
		out[i] = b.msgType
	}
	return out
}

func lastRedirect(t *testing.T, s *recordSession) message.Redirect {
	t.Helper()
	for i := len(s.broadcasts) - 1; i >= 0; i-- {
		if s.broadcasts[i].msgType == message.OutRedirect {
			return s.broadcasts[i].payload.(message.Redirect)
		}
	}
	t.Fatal("no tracking.redirect broadcast")
	return message.Redirect{}
}

func waitRedirect(t *testing.T, coord *TrackingCoordinator, s *recordSession, at time.Time, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		coord.applyRedirectResults(s, at)
		if done() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("router answer never reached the room")
}

func TestThreeOffRouteSamplesEnterDeviatedAndSpendALife(t *testing.T) {
	router := &stubRouter{polyline: polyline.Encode([]polyline.Point{{Lat: beside.Lat, Lng: beside.Lng}, {Lat: stopB.Lat, Lng: stopB.Lng}})}
	coord, session, p := deviationFixture(router)
	t0 := time.Now()
	step := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }

	feed(coord, session, p, step(1), beside, true)
	feed(coord, session, p, step(2), beside, true)
	feed(coord, session, p, step(3), stopA, false) // resets the count
	feed(coord, session, p, step(4), beside, true)
	feed(coord, session, p, step(5), beside, true)
	if p.DriveStatus != enum.OnROUTE || p.Lives != 3 || len(session.broadcasts) != 0 {
		t.Fatalf("status %s lives %d broadcasts %v", p.DriveStatus, p.Lives, msgTypes(session))
	}

	// The same snap counted twice changes nothing.
	coord.applyDeviation(session, p, step(5))
	if p.OffRouteSamples != 2 {
		t.Fatalf("samples %d", p.OffRouteSamples)
	}

	feed(coord, session, p, step(6), beside, true)
	if p.DriveStatus != enum.DEVIATED || p.Lives != 2 {
		t.Fatalf("status %s lives %d", p.DriveStatus, p.Lives)
	}
	if !p.ImmuneUntil.Equal(step(6).Add(engine.ImmuneDuration)) {
		t.Fatalf("immune until %v", p.ImmuneUntil)
	}
	types := msgTypes(session)
	if len(types) != 3 || types[0] != message.OutOffRoute || types[1] != message.OutLife || types[2] != message.OutRedirect {
		t.Fatalf("broadcasts %v", types)
	}
	off := session.broadcasts[0].payload.(message.OffRoute)
	if !off.Deviated || off.Lives != 2 || off.DistanceM != 110 || off.UserID != p.UserID {
		t.Fatalf("offRoute %+v", off)
	}
	life := session.broadcasts[1].payload.(message.Life)
	if life.Lives != 2 || life.Exhausted {
		t.Fatalf("life %+v", life)
	}
	pending := lastRedirect(t, session)
	if pending.Status != message.RedirectPending || pending.Reason != message.RedirectReasonConfirmed || pending.Target == nil || pending.Target.Name != "B" {
		t.Fatalf("redirect %+v", pending)
	}

	waitRedirect(t, coord, session, step(7), func() bool { return p.Redirect != nil && p.Redirect.Route != nil })
	ready := lastRedirect(t, session)
	if ready.Status != message.RedirectReady || ready.Polyline != router.polyline || ready.DistanceM != 120 || ready.DurationS != 40 {
		t.Fatalf("ready %+v", ready)
	}
	if router.calls.Load() != 1 {
		t.Fatalf("router calls %d", router.calls.Load())
	}
}

func TestTwoOnRouteSamplesRejoinAndImmunityKeepsTheLife(t *testing.T) {
	coord, session, p := deviationFixture(&stubRouter{polyline: "abc"})
	t0 := time.Now()
	step := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }

	for i := 1; i <= 3; i++ {
		feed(coord, session, p, step(i), beside, true)
	}
	if p.DriveStatus != enum.DEVIATED || p.Lives != 2 {
		t.Fatalf("status %s lives %d", p.DriveStatus, p.Lives)
	}

	feed(coord, session, p, step(4), stopA, false)
	feed(coord, session, p, step(5), beside, true) // resets the on-route count
	feed(coord, session, p, step(6), stopA, false)
	if p.DriveStatus != enum.DEVIATED {
		t.Fatalf("rejoined too early: %s", p.DriveStatus)
	}
	session.broadcasts = nil
	feed(coord, session, p, step(7), stopA, false)
	if p.DriveStatus != enum.OnROUTE || p.Redirect != nil {
		t.Fatalf("status %s redirect %+v", p.DriveStatus, p.Redirect)
	}
	types := msgTypes(session)
	if len(types) != 2 || types[0] != message.OutOffRoute || types[1] != message.OutRedirect {
		t.Fatalf("broadcasts %v", types)
	}
	if off := session.broadcasts[0].payload.(message.OffRoute); off.Deviated {
		t.Fatalf("offRoute %+v", off)
	}
	if cleared := lastRedirect(t, session); cleared.Status != message.RedirectCleared || cleared.Reason != message.RedirectReasonRejoined {
		t.Fatalf("cleared %+v", cleared)
	}

	// Deviate again inside the 30 s window: no life is spent.
	session.broadcasts = nil
	for i := 8; i <= 10; i++ {
		feed(coord, session, p, step(i), beside, true)
	}
	if p.DriveStatus != enum.DEVIATED || p.Lives != 2 {
		t.Fatalf("immune deviation: status %s lives %d", p.DriveStatus, p.Lives)
	}
	for _, typ := range msgTypes(session) {
		if typ == message.OutLife {
			t.Fatalf("life broadcast while immune: %v", msgTypes(session))
		}
	}

	// After the window a new deviation costs a life again.
	feed(coord, session, p, step(11), stopA, false)
	feed(coord, session, p, step(12), stopA, false)
	for i := 0; i < 3; i++ {
		feed(coord, session, p, step(40+i), beside, true)
	}
	if p.DriveStatus != enum.DEVIATED || p.Lives != 1 {
		t.Fatalf("after immunity: status %s lives %d", p.DriveStatus, p.Lives)
	}
}

func TestStaleRouterAnswerIsIgnored(t *testing.T) {
	coord, session, p := deviationFixture(&stubRouter{polyline: polyline.Encode([]polyline.Point{{Lat: beside.Lat, Lng: beside.Lng}, {Lat: stopB.Lat, Lng: stopB.Lng}})})
	t0 := time.Now()
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, t0.Add(time.Duration(i)*time.Second), beside, true)
	}
	waitRedirect(t, coord, session, t0, func() bool { return p.Redirect != nil && p.Redirect.Route != nil })
	current := p.Redirect.Generation
	route := p.Redirect.Route
	before := len(session.broadcasts)

	p.OfferRedirectResult(engine.RedirectResult{Generation: current - 1, Polyline: "stale"})
	coord.applyRedirectResults(session, t0)
	if p.Redirect.Route != route || p.Redirect.Polyline == "stale" || len(session.broadcasts) != before {
		t.Fatalf("stale answer applied: %+v", p.Redirect)
	}

	p.OfferRedirectResult(engine.RedirectResult{Generation: current - 1, Err: errRouterDown})
	coord.applyRedirectResults(session, t0)
	if p.Redirect.Failed || len(session.broadcasts) != before {
		t.Fatalf("stale error applied: %+v", p.Redirect)
	}
}

func TestFailedRedirectRetriesExactlyOnce(t *testing.T) {
	router := &stubRouter{err: errRouterDown}
	coord, session, p := deviationFixture(router)
	t0 := time.Now()
	step := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, step(i), beside, true)
	}
	waitRedirect(t, coord, session, step(3), func() bool { return p.Redirect != nil && p.Redirect.Failed })
	failed := lastRedirect(t, session)
	if failed.Status != message.RedirectFailed || failed.Reason != message.RedirectReasonProviderError {
		t.Fatalf("failed %+v", failed)
	}
	if !p.Redirect.RetryAt.Equal(step(3).Add(engine.RedirectRetryAfter)) {
		t.Fatalf("retryAt %v", p.Redirect.RetryAt)
	}

	// Not due yet: the sample is counted but nothing is sent.
	feed(coord, session, p, step(5), beside, true)
	if router.calls.Load() != 1 {
		t.Fatalf("retried early: %d calls", router.calls.Load())
	}

	feed(coord, session, p, step(14), beside, true)
	if !p.Redirect.Pending || p.Redirect.Retries != 1 {
		t.Fatalf("retry state %+v", p.Redirect)
	}
	waitRedirect(t, coord, session, step(14), func() bool { return p.Redirect.Failed })
	if router.calls.Load() != 2 || !p.Redirect.RetryAt.IsZero() {
		t.Fatalf("second failure: calls %d state %+v", router.calls.Load(), p.Redirect)
	}

	feed(coord, session, p, step(60), beside, true)
	coord.applyRedirectResults(session, step(60))
	if router.calls.Load() != 2 || p.DriveStatus != enum.DEVIATED {
		t.Fatalf("third attempt happened: calls %d", router.calls.Load())
	}
}

func TestMovingFarEnoughRefreshesTheRedirect(t *testing.T) {
	router := &stubRouter{polyline: polyline.Encode([]polyline.Point{{Lat: beside.Lat, Lng: beside.Lng}, {Lat: stopB.Lat, Lng: stopB.Lng}})}
	coord, session, p := deviationFixture(router)
	t0 := time.Now()
	step := func(i int) time.Time { return t0.Add(time.Duration(i) * time.Second) }
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, step(i), beside, true)
	}
	waitRedirect(t, coord, session, step(3), func() bool { return p.Redirect != nil && p.Redirect.Route != nil })
	first := p.Redirect
	oldRoute := first.Route

	// Moved 110 m but the cooldown has not passed.
	moved := model.Point{Lat: beside.Lat + 0.001, Lng: beside.Lng}
	feed(coord, session, p, step(10), moved, true)
	if p.Redirect != first || router.calls.Load() != 1 {
		t.Fatalf("refreshed before cooldown: calls %d", router.calls.Load())
	}

	feed(coord, session, p, step(30), moved, true)
	if p.Redirect == first || !p.Redirect.Pending || p.Redirect.Reason != message.RedirectReasonRefresh {
		t.Fatalf("refresh state %+v", p.Redirect)
	}
	if p.Redirect.Route != oldRoute || p.Redirect.Retries != 0 || p.Redirect.Generation != first.Generation+1 {
		t.Fatalf("refresh did not keep the old route: %+v", p.Redirect)
	}
	if pending := lastRedirect(t, session); pending.Status != message.RedirectPending || pending.Reason != message.RedirectReasonRefresh {
		t.Fatalf("pending %+v", pending)
	}
}

func TestRedirectArrivalAdvancesTheHop(t *testing.T) {
	coord, session, p := deviationFixture(&stubRouter{polyline: "abc"})
	t0 := time.Now()
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, t0.Add(time.Duration(i)*time.Second), beside, true)
	}
	if p.DriveStatus != enum.DEVIATED || p.Redirect == nil || p.Redirect.Hop != 0 {
		t.Fatalf("status %s redirect %+v", p.DriveStatus, p.Redirect)
	}
	defer p.StopSnap()

	session.broadcasts = nil
	p.RedirectArrived = true
	p.RedirectDistM = 4
	feed(coord, session, p, t0.Add(10*time.Second), stopB, true)
	if p.DriveStatus != enum.OnROUTE || p.HopIndex != 1 || p.Redirect != nil || p.RedirectArrived {
		t.Fatalf("after arrival: status %s hop %d redirect %+v", p.DriveStatus, p.HopIndex, p.Redirect)
	}
	if p.OffRouteSamples != 0 || p.OnRouteSamples != 0 {
		t.Fatalf("counters %d/%d", p.OffRouteSamples, p.OnRouteSamples)
	}
	types := msgTypes(session)
	if len(types) != 2 || types[0] != message.OutOffRoute || types[1] != message.OutRedirect {
		t.Fatalf("broadcasts %v", types)
	}
	off := session.broadcasts[0].payload.(message.OffRoute)
	if off.Deviated || off.HopIndex != 1 || off.DistanceM != 4 {
		t.Fatalf("offRoute %+v", off)
	}
	if cleared := lastRedirect(t, session); cleared.Status != message.RedirectCleared || cleared.Reason != message.RedirectReasonArrivedStop {
		t.Fatalf("cleared %+v", cleared)
	}
}

func TestRedirectArrivalAtTheLastStopCompletesTheRider(t *testing.T) {
	coord, session, p := deviationFixture(&stubRouter{polyline: "abc"})
	p.HopIndex = 1
	t0 := time.Now()
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, t0.Add(time.Duration(i)*time.Second), model.Point{Lat: 10.785, Lng: 106.701}, true)
	}
	if p.Redirect == nil || p.Redirect.Hop != 1 || p.Redirect.Target.Name != "C" {
		t.Fatalf("redirect %+v", p.Redirect)
	}

	p.RedirectArrived = true
	feed(coord, session, p, t0.Add(10*time.Second), stopC, true)
	if p.DriveStatus != enum.COMPLETED || p.Redirect != nil {
		t.Fatalf("status %s redirect %+v", p.DriveStatus, p.Redirect)
	}
	coord.OnTick(session, t0.Add(11*time.Second))
	if coord.tripInformation.Status != enum.TripCompleted {
		t.Fatalf("trip status %s", coord.tripInformation.Status)
	}
}

func TestLastHopHasNoNextStopToRedirectTo(t *testing.T) {
	coord, session, p := deviationFixture(&stubRouter{polyline: "abc"})
	coord.branch(p.BranchID).Hops[0].Ready = false
	t0 := time.Now()
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, t0.Add(time.Duration(i)*time.Second), beside, true)
	}
	failed := lastRedirect(t, session)
	if failed.Status != message.RedirectFailed || failed.Reason != message.RedirectReasonHopNotReady || failed.Target != nil {
		t.Fatalf("failed %+v", failed)
	}
	if p.DriveStatus != enum.DEVIATED || !p.Redirect.RetryAt.IsZero() {
		t.Fatalf("status %s redirect %+v", p.DriveStatus, p.Redirect)
	}
}

func TestNilRouterReportsProviderError(t *testing.T) {
	coord, session, p := deviationFixture(nil)
	t0 := time.Now()
	for i := 1; i <= 3; i++ {
		feed(coord, session, p, t0.Add(time.Duration(i)*time.Second), beside, true)
	}
	failed := lastRedirect(t, session)
	if failed.Status != message.RedirectFailed || failed.Reason != message.RedirectReasonProviderError {
		t.Fatalf("failed %+v", failed)
	}
}

func TestEndStayRestoresADeviatedRider(t *testing.T) {
	leader := uuid.New()
	member := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{
		leader: enum.OnROUTE,
		member: enum.DEVIATED,
	})
	coord.progresses[member].OffRouteSamples = 2

	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`true`))
	if coord.progresses[member].DriveStatus != enum.StayPrivate || coord.progresses[member].OffRouteSamples != 0 {
		t.Fatalf("stay %+v", coord.progresses[member])
	}
	coord.OnMessage(session, member, string(message.UserStayPublic), json.RawMessage(`true`))

	coord.OnMessage(session, leader, string(message.StayMode), json.RawMessage(`false`))
	if coord.progresses[member].DriveStatus != enum.DEVIATED || coord.progresses[member].HasResume {
		t.Fatalf("member resumed as %s", coord.progresses[member].DriveStatus)
	}
	if coord.progresses[leader].DriveStatus != enum.OnROUTE {
		t.Fatalf("leader resumed as %s", coord.progresses[leader].DriveStatus)
	}
}

func TestPositionSamplesFasterThanTheLimitAreDropped(t *testing.T) {
	leader := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{leader: enum.OnROUTE})

	coord.OnMessage(session, leader, string(message.Position), json.RawMessage(`{"Lat":11.1,"Lng":106.2}`))
	coord.OnMessage(session, leader, string(message.Position), json.RawMessage(`{"Lat":12.5,"Lng":107.9}`))
	prg := coord.progresses[leader]
	if prg.Lat != 11.1 || prg.Lng != 106.2 {
		t.Fatalf("second sample was accepted: %+v", prg)
	}

	prg.ObservedAt = time.Now().Add(-engine.SampleMinInterval)
	coord.OnMessage(session, leader, string(message.Position), json.RawMessage(`{"Lat":12.5,"Lng":107.9}`))
	if prg.Lat != 12.5 || prg.Lng != 107.9 {
		t.Fatalf("late sample was dropped: %+v", prg)
	}
}

func TestPositionsBroadcastUsesCamelCase(t *testing.T) {
	leader := uuid.New()
	coord, session := stayFixture(leader, map[uuid.UUID]enum.DriveStatus{leader: enum.DEVIATED})
	coord.OnTick(session, time.Now())
	if len(session.broadcasts) != 1 {
		t.Fatalf("broadcasts %v", msgTypes(session))
	}
	raw, err := json.Marshal(session.broadcasts[0].payload)
	if err != nil {
		t.Fatal(err)
	}
	var posted []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &posted); err != nil {
		t.Fatal(err)
	}
	if len(posted) != 1 {
		t.Fatalf("posted %s", raw)
	}
	for _, key := range []string{"userId", "lat", "lng", "batteryPercent", "signalBars", "driveStatus", "connectionStatus"} {
		if _, ok := posted[0][key]; !ok {
			t.Fatalf("missing %q in %s", key, raw)
		}
	}
	if string(posted[0]["driveStatus"]) != `"DEVIATED"` {
		t.Fatalf("driveStatus %s", posted[0]["driveStatus"])
	}
}
