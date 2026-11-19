package tracking

import (
	"errors"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"
)

// Queueing and provider work together have a bounded age. A lost/unpublished
// request becomes a normal failed redirect instead of staying pending forever.
const redirectJobMaxAge = 30 * time.Second

func (t *TrackingCoordinator) submitRedirect(p *engine.TravelProgress) bool {
	state := p.Redirect
	return t.recovery.Redirects.TrySubmit(model.RedirectJob{
		JobID: state.JobID, TripID: t.tripInformation.TripId, MemberID: p.UserID,
		Generation: state.Generation, SchemaVersion: model.RedirectSchemaVersion,
		Origin: state.Origin, Target: model.Point{Lat: state.Target.Latitude, Lng: state.Target.Longitude},
		ExpiresAt: state.RequestedAt.Add(redirectJobMaxAge),
	})
}

func (t *TrackingCoordinator) drainRemoteRedirectResults() {
	for {
		select {
		case result, ok := <-t.redirectResults:
			if !ok {
				t.redirectResults = nil
				return
			}
			p := t.progresses[result.MemberID]
			if p == nil || p.Redirect == nil || result.TripID != t.tripInformation.TripId ||
				result.JobID != p.Redirect.JobID || result.Generation != p.Redirect.Generation || !p.Redirect.Pending {
				continue
			}
			var err error
			if result.Error != "" {
				err = errors.New(result.Error)
			}
			p.OfferRedirectResult(engine.RedirectResult{
				Generation: result.Generation, Polyline: result.Polyline,
				DistanceM: result.DistanceM, DurationS: result.DurationS, Err: err,
			})
		default:
			return
		}
	}
}

func (t *TrackingCoordinator) advanceRedirectJobs(s realtime.Session, now time.Time) {
	for _, p := range t.progresses {
		if p.AwaitingFreshPosition {
			continue
		}
		if p.ResumeRedirect && p.DriveStatus == enum.DEVIATED && t.tripInformation.Status == enum.TripActive {
			p.ResumeRedirect = false
			t.startRedirect(s, p, now, message.RedirectReasonRefresh, p.Redirect)
		}
		state := p.Redirect
		if t.recovery.Redirects == nil || state == nil || !state.Pending {
			continue
		}
		if now.Sub(state.RequestedAt) >= redirectJobMaxAge {
			t.failRedirect(s, p, state, message.RedirectReasonProviderError, now, true)
			continue
		}
		if !state.Submitted {
			state.Submitted = t.submitRedirect(p)
		}
	}
}
