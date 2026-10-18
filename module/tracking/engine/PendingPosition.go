package engine

import (
	"context"
	"log"
	"sync/atomic"
	"time"

	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"
)

// activeSnapWorkers is how many snap loops are still inside runSnap.
// Tests use it to show a restart does not leave the previous loop blocked.
var activeSnapWorkers atomic.Int32

// snapJob is a sample copied on the room goroutine. The worker never reads TravelProgress.
// redirect is the temporary route to measure against while deviated; nil otherwise.
type snapJob struct {
	point       model.Point
	observedAt  time.Time
	drive       enum.DriveStatus
	redirect    *geo.Route
	redirectGen uint64
}

// SnapUpdate is the worker's result. Only the room goroutine copies it onto TravelProgress.
// Finished stays set once the rider reaches the end of the last hop, so a newer
// update cannot discard the arrival.
//
// DistM and OffRoute describe the sample against the branch polyline. The
// Redirect fields describe the same sample against the temporary route, and
// RedirectGen says which route that was.
type SnapUpdate struct {
	HopIndex     int
	LastSegIndex int
	Progress     float64
	Route        *geo.Route
	Finished     bool

	ObservedAt time.Time
	DistM      float64
	OffRoute   bool

	RedirectGen      uint64
	RedirectSeg      int
	RedirectProgress float64
	RedirectDistM    float64
	RedirectArrived  bool
}

// hopCursor is the worker's private copy of hop state.
type hopCursor struct {
	hop      int
	seg      int
	progress float64
	route    *geo.Route
	finished bool

	observedAt time.Time
	dist       float64
	offRoute   bool

	redirectRoute    *geo.Route
	redirectGen      uint64
	redirectSeg      int
	redirectProgress float64
	redirectDist     float64
	redirectArrived  bool
}

func (c hopCursor) update() SnapUpdate {
	return SnapUpdate{
		HopIndex:         c.hop,
		LastSegIndex:     c.seg,
		Progress:         c.progress,
		Route:            c.route,
		Finished:         c.finished,
		ObservedAt:       c.observedAt,
		DistM:            c.dist,
		OffRoute:         c.offRoute,
		RedirectGen:      c.redirectGen,
		RedirectSeg:      c.redirectSeg,
		RedirectProgress: c.redirectProgress,
		RedirectDistM:    c.redirectDist,
		RedirectArrived:  c.redirectArrived,
	}
}

// StartSnap stops any previous loop, then starts one that reports SnapUpdate values.
// Call it from the room goroutine. A nil branch leaves the rider with no loop.
func (p *TravelProgress) StartSnap(branch *model.TravelBranch) {
	p.ApplySnap()
	p.StopSnap()
	if branch == nil {
		log.Printf("branch %s not found", p.BranchID)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	jobs := make(chan snapJob, 1)
	updates := make(chan SnapUpdate, 1)
	p.cancel = cancel
	p.jobs = jobs
	p.updates = updates
	cursor := hopCursor{hop: p.HopIndex, seg: p.LastSegIndex, progress: p.Progress, route: p.Route}
	if p.Redirect != nil && p.Redirect.Route != nil {
		cursor.redirectRoute = p.Redirect.Route
		cursor.redirectGen = p.Redirect.Generation
		cursor.redirectSeg = p.RedirectSeg
		cursor.redirectProgress = p.RedirectProgress
	}
	go p.runSnap(ctx, jobs, updates, branch, cursor)
}

// StopSnap cancels the loop. It does not close the job channel: the loop
// exits on ctx.Done, so a later submit cannot send on a closed channel.
func (p *TravelProgress) StopSnap() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.jobs = nil
	p.updates = nil
}

// ApplySnap copies every update the worker has already published.
func (p *TravelProgress) ApplySnap() {
	if p.updates == nil {
		return
	}

	for {
		select {
		case update := <-p.updates:
			p.HopIndex = update.HopIndex
			p.LastSegIndex = update.LastSegIndex
			p.Progress = update.Progress
			p.Route = update.Route
			p.LastSnapAt = update.ObservedAt
			p.LastDistM = update.DistM
			p.OffRoute = update.OffRoute
			if p.Redirect != nil && update.RedirectGen == p.Redirect.Generation {
				p.RedirectSeg = update.RedirectSeg
				p.RedirectProgress = update.RedirectProgress
				p.RedirectDistM = update.RedirectDistM
				p.RedirectArrived = update.RedirectArrived
			}
			if update.Finished {
				p.DriveStatus = enum.COMPLETED
			}
		default:
			return
		}
	}
}

// SubmitSample queues the latest lat/lng. A full buffer drops the sample.
// The next tick still has the newest coordinates on TravelProgress.
func (p *TravelProgress) SubmitSample() {
	if p.jobs == nil || p.ObservedAt.IsZero() {
		return
	}
	job := snapJob{
		point:      model.Point{Lat: p.Lat, Lng: p.Lng},
		observedAt: p.ObservedAt,
		drive:      p.DriveStatus,
	}
	if p.DriveStatus == enum.DEVIATED && p.Redirect != nil && p.Redirect.Route != nil {
		job.redirect = p.Redirect.Route
		job.redirectGen = p.Redirect.Generation
	}
	select {
	case p.jobs <- job:
	default:
	}
}

func (p *TravelProgress) runSnap(ctx context.Context, jobs <-chan snapJob, updates chan SnapUpdate, branch *model.TravelBranch, cursor hopCursor) {
	activeSnapWorkers.Add(1)
	defer activeSnapWorkers.Add(-1)
	defer func() {
		if err := recover(); err != nil {
			log.Printf("tracking snap: %v", err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-jobs:
			if job.drive == enum.STOP {
				continue
			}
			next, ok := snapPointOnBranch(cursor, branch, job.point)
			if !ok {
				return
			}
			cursor = snapPointOnRedirect(next, job.redirect, job.redirectGen, job.point)
			cursor.observedAt = job.observedAt
			sendLatestUpdate(ctx, updates, cursor.update())
			if cursor.finished {
				return
			}
		}
	}
}

// sendLatestUpdate puts update into a buffer of one.
// If an older SnapUpdate is still sitting there, it is discarded so the room
// goroutine applies the newest snap on the next tick.
func sendLatestUpdate(ctx context.Context, updates chan SnapUpdate, update SnapUpdate) {
	select {
	case <-ctx.Done():
		return
	case updates <- update:
		return
	default:
	}
	select {
	case <-updates:
	default:
	}
	select {
	case <-ctx.Done():
	case updates <- update:
	default:
	}
}

// snapPointOnBranch snaps the GPS point onto the current hop polyline.
// A point farther than geo.OffRouteM keeps the cursor where it was and only
// reports the distance: an off-road sample must not drag the rider along the
// line. A point at the end of the hop moves the cursor onto the next hop.
// false means this hop has no usable polyline, so the snap loop should stop.
func snapPointOnBranch(cursor hopCursor, branch *model.TravelBranch, point model.Point) (hopCursor, bool) {
	if cursor.finished {
		return cursor, true
	}
	if cursor.route == nil || len(cursor.route.Points) < 2 {
		if !cursor.loadHopPolyline(branch, cursor.hop, 0) {
			return cursor, false
		}
	}
	snap := cursor.route.Snap(point, cursor.seg)
	cursor.dist = snap.Dist
	if snap.Dist > geo.OffRouteM {
		cursor.offRoute = true
		return cursor, true
	}
	cursor.offRoute = false
	cursor.seg = snap.Seg
	cursor.progress = snap.Progress
	if !cursor.route.AtEnd(snap.Seg, snap.Progress, snap.Dist) {
		return cursor, true
	}
	if cursor.hop+1 >= len(branch.Hops) {
		cursor.finished = true
		return cursor, true
	}
	base := cursor.route.TotalM()
	if !cursor.loadHopPolyline(branch, cursor.hop+1, base) {
		return cursor, true
	}
	return cursor, true
}

// snapPointOnRedirect measures the same sample against the temporary route.
// It never touches the branch cursor. A new route pointer resets the redirect
// cursor to its first segment. nil clears the redirect fields.
func snapPointOnRedirect(cursor hopCursor, route *geo.Route, gen uint64, point model.Point) hopCursor {
	if route == nil {
		cursor.redirectRoute = nil
		cursor.redirectGen = 0
		cursor.redirectSeg = 0
		cursor.redirectProgress = 0
		cursor.redirectDist = 0
		cursor.redirectArrived = false
		return cursor
	}
	if cursor.redirectRoute != route {
		cursor.redirectRoute = route
		cursor.redirectSeg = 0
		cursor.redirectProgress = 0
		cursor.redirectArrived = false
	}
	// A refresh keeps the old route on screen until the new one answers, so
	// the generation can move while the pointer stays the same.
	cursor.redirectGen = gen
	snap := route.Snap(point, cursor.redirectSeg)
	cursor.redirectDist = snap.Dist
	if snap.Dist > geo.OffRouteM {
		return cursor
	}
	cursor.redirectSeg = snap.Seg
	cursor.redirectProgress = snap.Progress
	if route.AtEnd(snap.Seg, snap.Progress, snap.Dist) {
		cursor.redirectArrived = true
	}
	return cursor
}

// loadHopPolyline builds the cursor route from branch.Hops[hopIndex].
// baseM is the meters already traveled on earlier hops, so progress keeps counting.
func (c *hopCursor) loadHopPolyline(branch *model.TravelBranch, hopIndex int, baseM float64) bool {
	if branch == nil || hopIndex < 0 || hopIndex >= len(branch.Hops) {
		log.Printf("invalid hop index %d", hopIndex)
		return false
	}
	hop := branch.Hops[hopIndex]
	if !hop.Ready || len(hop.Points) < 2 {
		log.Printf("hop %d is not ready", hopIndex)
		return false
	}
	next := geo.NewRoute(hop.Points)
	next.Shift(baseM)
	c.hop = hopIndex
	c.seg = 0
	c.route = next
	return true
}
