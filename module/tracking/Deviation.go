package tracking

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

// applyDeviation runs the OnROUTE / DEVIATED state machine for one rider on
// the room goroutine. It counts one sample per snap update, so a tick that
// saw no new GPS fix changes nothing.
func (t *TrackingCoordinator) applyDeviation(s realtime.Session, p *engine.TravelProgress, now time.Time) {
	if t.tripInformation == nil || t.tripInformation.Status != enum.TripActive {
		return
	}
	if p.DriveStatus != enum.OnROUTE && p.DriveStatus != enum.DEVIATED {
		return
	}
	if p.LastSnapAt.IsZero() || !p.LastSnapAt.After(p.LastCountedAt) {
		return
	}
	p.LastCountedAt = p.LastSnapAt

	if p.DriveStatus == enum.DEVIATED && p.Redirect != nil && p.RedirectArrived {
		t.arriveByRedirect(s, p)
		return
	}

	if p.OffRoute {
		p.OffRouteSamples++
		p.OnRouteSamples = 0
	} else {
		p.OnRouteSamples++
		p.OffRouteSamples = 0
	}

	switch p.DriveStatus {
	case enum.OnROUTE:
		if p.OffRouteSamples >= engine.ConfirmSamples {
			t.enterDeviated(s, p, now)
		}
	case enum.DEVIATED:
		if p.OnRouteSamples >= engine.ClearSamples {
			t.rejoin(s, p)
			return
		}
		t.maybeRefreshRedirect(s, p, now)
	}
}

// enterDeviated flips the rider to DEVIATED, spends a life unless immune, and
// asks for a temporary route.
func (t *TrackingCoordinator) enterDeviated(s realtime.Session, p *engine.TravelProgress, now time.Time) {
	p.DriveStatus = enum.DEVIATED
	p.OffRouteSamples = 0
	p.OnRouteSamples = 0

	lifeLost := false
	if now.Before(p.ImmuneUntil) {
		// still immune from the previous deviation
	} else if p.Lives > 0 {
		p.Lives--
		p.ImmuneUntil = now.Add(engine.ImmuneDuration)
		lifeLost = true
	}

	s.Broadcast(message.OutOffRoute, message.OffRoute{
		UserID:    p.UserID,
		Deviated:  true,
		DistanceM: p.LastDistM,
		BranchID:  p.BranchID,
		HopIndex:  p.HopIndex,
		Lives:     p.Lives,
	})
	if lifeLost {
		s.Broadcast(message.OutLife, message.Life{
			UserID:      p.UserID,
			Lives:       p.Lives,
			ImmuneUntil: p.ImmuneUntil,
			Exhausted:   p.Lives == 0,
		})
	}
	t.startRedirect(s, p, now, message.RedirectReasonConfirmed, nil)
}

// rejoin returns a deviated rider to OnROUTE after enough samples back on the
// branch polyline. The branch cursor was already released by the worker.
func (t *TrackingCoordinator) rejoin(s realtime.Session, p *engine.TravelProgress) {
	p.DriveStatus = enum.OnROUTE
	p.OffRouteSamples = 0
	p.OnRouteSamples = 0
	s.Broadcast(message.OutOffRoute, message.OffRoute{
		UserID:    p.UserID,
		Deviated:  false,
		DistanceM: p.LastDistM,
		BranchID:  p.BranchID,
		HopIndex:  p.HopIndex,
		Lives:     p.Lives,
	})
	t.clearRedirect(s, p, message.RedirectReasonRejoined)
}

// arriveByRedirect handles the end of the temporary route: the rider reached
// the next stop without touching the branch polyline. The next hop starts
// fresh; the last stop completes the branch.
func (t *TrackingCoordinator) arriveByRedirect(s realtime.Session, p *engine.TravelProgress) {
	targetHop := p.Redirect.Hop + 1
	branch := t.branch(p.BranchID)
	p.OffRouteSamples = 0
	p.OnRouteSamples = 0

	// Stop the worker first so a stale update cannot pull the hop back.
	p.StopSnap()
	s.Broadcast(message.OutOffRoute, message.OffRoute{
		UserID:    p.UserID,
		Deviated:  false,
		DistanceM: p.RedirectDistM,
		BranchID:  p.BranchID,
		HopIndex:  targetHop,
		Lives:     p.Lives,
	})
	t.clearRedirect(s, p, message.RedirectReasonArrivedStop)

	if branch == nil || targetHop >= len(branch.Hops) {
		p.DriveStatus = enum.COMPLETED
		return
	}
	p.DriveStatus = enum.OnROUTE
	if p.HopIndex < targetHop {
		p.HopIndex = targetHop
		p.LastSegIndex = 0
		p.Progress = 0
		p.Route = nil
	}
	p.StartSnap(branch)
}

// clearRedirect forgets the temporary route and tells the room.
func (t *TrackingCoordinator) clearRedirect(s realtime.Session, p *engine.TravelProgress, reason string) {
	origin := message.RedirectPoint{Lat: p.Lat, Lng: p.Lng}
	var target *message.RedirectTarget
	if p.Redirect != nil {
		origin = message.RedirectPoint{Lat: p.Redirect.Origin.Lat, Lng: p.Redirect.Origin.Lng}
		target = redirectTarget(p.Redirect)
	}
	p.ClearRedirect()
	s.Broadcast(message.OutRedirect, message.Redirect{
		UserID: p.UserID,
		Status: message.RedirectCleared,
		Reason: reason,
		Origin: origin,
		Target: target,
	})
}

// startRedirect opens one router call toward the next stop of the current hop.
// prev is the attempt being replaced; a refresh keeps its route on screen until
// the new answer arrives.
func (t *TrackingCoordinator) startRedirect(s realtime.Session, p *engine.TravelProgress, now time.Time, reason string, prev *engine.RedirectState) {
	gen := p.NextRedirectGeneration()
	origin := model.Point{Lat: p.Lat, Lng: p.Lng}
	state := &engine.RedirectState{
		Generation:  gen,
		Reason:      reason,
		Hop:         p.HopIndex,
		Origin:      origin,
		RequestedAt: now,
		Pending:     true,
	}
	if prev != nil && prev.Hop == p.HopIndex {
		state.Target = prev.Target
		state.Polyline = prev.Polyline
		state.Route = prev.Route
		state.DistanceM = prev.DistanceM
		state.DurationS = prev.DurationS
		if reason != message.RedirectReasonRefresh {
			state.Retries = prev.Retries + 1
		}
	} else {
		p.RedirectSeg = 0
		p.RedirectProgress = 0
		p.RedirectDistM = 0
		p.RedirectArrived = false
	}
	p.Redirect = state

	if state.Target.DestinationID == uuid.Nil {
		target, ok := nextStop(t.branch(p.BranchID), p.HopIndex)
		if !ok {
			t.failRedirect(s, p, state, message.RedirectReasonHopNotReady, now, false)
			return
		}
		state.Target = target
	}
	if t.router == nil {
		t.failRedirect(s, p, state, message.RedirectReasonProviderError, now, false)
		return
	}

	s.Broadcast(message.OutRedirect, message.Redirect{
		UserID: p.UserID,
		Status: message.RedirectPending,
		Reason: reason,
		Origin: message.RedirectPoint{Lat: origin.Lat, Lng: origin.Lng},
		Target: redirectTarget(state),
	})
	go t.callRouter(p, gen, origin, state.Target)
}

// callRouter runs off the room goroutine. It only touches the router and the
// rider's result buffer, never TravelProgress fields.
func (t *TrackingCoordinator) callRouter(p *engine.TravelProgress, gen uint64, from model.Point, to model.Stop) {
	ctx, cancel := context.WithTimeout(context.Background(), engine.RedirectTimeout)
	defer cancel()
	polyline, distanceM, durationS, err := t.router.Redirect(ctx, from.Lat, from.Lng, to.Latitude, to.Longitude)
	p.OfferRedirectResult(engine.RedirectResult{
		Generation: gen,
		Polyline:   polyline,
		DistanceM:  distanceM,
		DurationS:  durationS,
		Err:        err,
	})
}

// applyRedirectResults copies router answers onto the rider. It runs every
// tick, even during stay, so a late answer never stays in the buffer.
func (t *TrackingCoordinator) applyRedirectResults(s realtime.Session, now time.Time) {
	for _, p := range t.progresses {
		for _, result := range p.DrainRedirectResults() {
			state := p.Redirect
			if state == nil || result.Generation != state.Generation {
				continue
			}
			deviated := p.DriveStatus == enum.DEVIATED || (p.HasResume && p.ResumeDrive == enum.DEVIATED)
			if !deviated {
				continue
			}
			state.Pending = false
			if result.Err != nil || result.Polyline == "" {
				t.failRedirect(s, p, state, message.RedirectReasonProviderError, now, true)
				continue
			}
			points, err := model.DecodePolyline(result.Polyline)
			if err != nil || len(points) < 2 {
				t.failRedirect(s, p, state, message.RedirectReasonProviderError, now, true)
				continue
			}
			state.Failed = false
			state.RetryAt = time.Time{}
			state.Polyline = result.Polyline
			state.Route = geo.NewRoute(points)
			state.DistanceM = result.DistanceM
			state.DurationS = result.DurationS
			p.RedirectSeg = 0
			p.RedirectProgress = 0
			p.RedirectDistM = 0
			p.RedirectArrived = false
			s.Broadcast(message.OutRedirect, message.Redirect{
				UserID:    p.UserID,
				Status:    message.RedirectReady,
				Reason:    state.Reason,
				Origin:    message.RedirectPoint{Lat: state.Origin.Lat, Lng: state.Origin.Lng},
				Target:    redirectTarget(state),
				Polyline:  state.Polyline,
				DistanceM: state.DistanceM,
				DurationS: state.DurationS,
			})
		}
	}
}

// failRedirect marks the attempt failed. A provider error schedules one retry;
// a missing stop does not, because nothing changes until the rider moves.
func (t *TrackingCoordinator) failRedirect(s realtime.Session, p *engine.TravelProgress, state *engine.RedirectState, reason string, now time.Time, retry bool) {
	state.Pending = false
	state.Failed = true
	state.RetryAt = time.Time{}
	if retry && state.Retries == 0 {
		state.RetryAt = now.Add(engine.RedirectRetryAfter)
	}
	s.Broadcast(message.OutRedirect, message.Redirect{
		UserID: p.UserID,
		Status: message.RedirectFailed,
		Reason: reason,
		Origin: message.RedirectPoint{Lat: state.Origin.Lat, Lng: state.Origin.Lng},
		Target: redirectTarget(state),
	})
}

// maybeRefreshRedirect recomputes the route once the rider has moved far
// enough from the origin and the cooldown has passed. A due retry runs when no
// refresh replaced it.
func (t *TrackingCoordinator) maybeRefreshRedirect(s realtime.Session, p *engine.TravelProgress, now time.Time) {
	state := p.Redirect
	if state == nil || state.Pending {
		return
	}
	current := model.Point{Lat: p.Lat, Lng: p.Lng}
	moved := geo.DistanceM(state.Origin, current) >= engine.RedirectRefreshMoveM
	cooled := now.Sub(state.RequestedAt) >= engine.RedirectRefreshCooldown
	if moved && cooled {
		t.startRedirect(s, p, now, message.RedirectReasonRefresh, state)
		return
	}
	if state.Failed && !state.RetryAt.IsZero() && !now.Before(state.RetryAt) {
		t.startRedirect(s, p, now, state.Reason, state)
	}
}

// nextStop is the far end of Hops[hop]. false when the branch has no such hop
// or its polyline is not ready.
func nextStop(branch *model.TravelBranch, hop int) (model.Stop, bool) {
	if branch == nil || hop < 0 || hop >= len(branch.Hops) || hop+1 >= len(branch.Stops) {
		return model.Stop{}, false
	}
	if !branch.Hops[hop].Ready {
		return model.Stop{}, false
	}
	return branch.Stops[hop+1], true
}

func redirectTarget(state *engine.RedirectState) *message.RedirectTarget {
	if state == nil || state.Target.DestinationID == uuid.Nil {
		return nil
	}
	return &message.RedirectTarget{
		DestinationID: state.Target.DestinationID,
		Name:          state.Target.Name,
		Lat:           state.Target.Latitude,
		Lng:           state.Target.Longitude,
	}
}
