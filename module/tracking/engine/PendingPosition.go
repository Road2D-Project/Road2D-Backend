package engine

import (
	"context"
	"log"
	"sync/atomic"

	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"
)

// activeSnapWorkers is how many snap loops are still inside runSnap.
// Tests use it to show a restart does not leave the previous loop blocked.
var activeSnapWorkers atomic.Int32

// snapJob is a sample copied on the room goroutine. The worker never reads TravelProgress.
type snapJob struct {
	point model.Point
	drive enum.DriveStatus
}

// SnapUpdate is the worker's result. Only the room goroutine copies it onto TravelProgress.
type SnapUpdate struct {
	HopIndex     int
	LastSegIndex int
	Progress     float64
	Route        *geo.Route
}

// hopCursor is the worker's private copy of hop state.
type hopCursor struct {
	hop      int
	seg      int
	progress float64
	route    *geo.Route
}

func (c hopCursor) update() SnapUpdate {
	return SnapUpdate{
		HopIndex:     c.hop,
		LastSegIndex: c.seg,
		Progress:     c.progress,
		Route:        c.route,
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
	go runSnap(ctx, jobs, updates, branch, cursor)
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
		point: model.Point{Lat: p.Lat, Lng: p.Lng},
		drive: p.DriveStatus,
	}
	select {
	case p.jobs <- job:
	default:
	}
}

func runSnap(ctx context.Context, jobs <-chan snapJob, updates chan SnapUpdate, branch *model.TravelBranch, cursor hopCursor) {
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
			cursor = next
			sendLatestUpdate(ctx, updates, cursor.update())
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
// A point at the end of the hop moves the cursor onto the next hop.
// false means this hop has no usable polyline, so the snap loop should stop.
func snapPointOnBranch(cursor hopCursor, branch *model.TravelBranch, point model.Point) (hopCursor, bool) {
	if cursor.route == nil || len(cursor.route.Points) < 2 {
		if !cursor.loadHopPolyline(branch, cursor.hop, 0) {
			return cursor, false
		}
	}
	snap := cursor.route.Snap(point, cursor.seg)
	cursor.seg = snap.Seg
	cursor.progress = snap.Progress
	if !cursor.route.AtEnd(snap.Seg, snap.Progress, snap.Dist) {
		return cursor, true
	}
	if cursor.hop+1 >= len(branch.Hops) {
		return cursor, true
	}
	base := cursor.route.TotalM()
	if !cursor.loadHopPolyline(branch, cursor.hop+1, base) {
		return cursor, true
	}
	return cursor, true
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
