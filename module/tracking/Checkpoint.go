package tracking

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/geo"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) markCheckpointDirty() {
	t.revision++
	t.checkpointDirty = true
}

func (t *TrackingCoordinator) offerCheckpoint(now time.Time, force bool) {
	if t.recovery.Writer == nil || !t.checkpointDirty || t.tripInformation == nil {
		return
	}
	interval := t.recovery.CheckpointInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	if !force && !t.lastCheckpointAt.IsZero() && now.Sub(t.lastCheckpointAt) < interval {
		return
	}
	t.recovery.Writer.Offer(t.captureCheckpoint(now))
	t.checkpointDirty = false
	t.lastCheckpointAt = now
}

func (t *TrackingCoordinator) captureCheckpoint(now time.Time) model.TripCheckpoint {
	checkpoint := model.TripCheckpoint{
		TripID: t.tripInformation.TripId, Sequence: t.checkpointSequence,
		Revision: t.revision, Status: t.tripInformation.Status, CapturedAt: now.UTC(),
		Riders: make(map[uuid.UUID]model.RiderCheckpoint, len(t.progresses)),
	}
	for id, p := range t.progresses {
		rider := model.RiderCheckpoint{
			BranchID: p.BranchID, HopIndex: p.HopIndex, LastSegIndex: p.LastSegIndex, Progress: p.Progress,
			DriveStatus: p.DriveStatus, Lat: p.Lat, Lng: p.Lng,
			BatteryPercent: p.BatteryPercent, SignalBars: p.SignalBars, ObservedAt: p.ObservedAt,
			Lives: p.Lives, ImmuneUntil: p.ImmuneUntil, ResumeDrive: p.ResumeDrive, HasResume: p.HasResume,
			RedirectGeneration: p.RedirectGeneration, RedirectSeg: p.RedirectSeg, RedirectProgress: p.RedirectProgress,
		}
		for destinationID := range p.Arrived {
			rider.Arrived = append(rider.Arrived, destinationID)
		}
		if state := p.Redirect; state != nil {
			rider.Redirect = &model.RedirectCheckpoint{
				Generation: state.Generation, Reason: state.Reason, Retries: state.Retries, Hop: state.Hop,
				Origin: state.Origin, Target: state.Target, Polyline: state.Polyline,
				DistanceM: state.DistanceM, DurationS: state.DurationS, RequestedAt: state.RequestedAt,
				RetryAt: state.RetryAt, Pending: state.Pending, Failed: state.Failed,
			}
		}
		checkpoint.Riders[id] = rider
	}
	return checkpoint
}

func (t *TrackingCoordinator) restoreCheckpoint(checkpoint model.TripCheckpoint) {
	if checkpoint.TripID != t.tripInformation.TripId {
		return
	}
	t.revision, t.checkpointSequence = checkpoint.Revision, checkpoint.Sequence
	t.tripInformation.Status = checkpoint.Status
	for id, saved := range checkpoint.Riders {
		member := t.tripInformation.Riders[id]
		if member == nil {
			continue
		}
		p := engine.NewTravelProgress(id, member.AssignedBranchID)
		p.ConnectionStatus = enum.DISCONNECTED
		p.AwaitingFreshPosition = true
		p.Lat, p.Lng, p.ObservedAt = saved.Lat, saved.Lng, saved.ObservedAt
		p.BatteryPercent, p.SignalBars = saved.BatteryPercent, saved.SignalBars
		p.Lives, p.ImmuneUntil = saved.Lives, saved.ImmuneUntil
		p.RedirectGeneration = saved.RedirectGeneration
		if checkpoint.Status == enum.TripStay {
			p.DriveStatus = enum.StayPrivate
			p.ResumeDrive, p.HasResume = enum.OnROUTE, true
		}
		branch := t.branch(p.BranchID)
		if saved.BranchID == p.BranchID && branch != nil && saved.HopIndex >= 0 && saved.HopIndex <= len(branch.Hops) {
			p.HopIndex, p.LastSegIndex, p.Progress = saved.HopIndex, saved.LastSegIndex, saved.Progress
			if p.HopIndex < len(branch.Hops) && branch.Hops[p.HopIndex].Ready {
				p.Route = geo.NewRoute(branch.Hops[p.HopIndex].Points)
				base := 0.0
				for _, hop := range branch.Hops[:p.HopIndex] {
					base += geo.NewRoute(hop.Points).TotalM()
				}
				p.Route.Shift(base)
			}
			p.DriveStatus, p.ResumeDrive, p.HasResume = saved.DriveStatus, saved.ResumeDrive, saved.HasResume
			p.Arrived = make(map[uuid.UUID]struct{}, len(saved.Arrived))
			for _, destinationID := range saved.Arrived {
				p.Arrived[destinationID] = struct{}{}
			}
			if saved.Redirect != nil {
				t.restoreRedirect(p, *saved.Redirect, saved.RedirectSeg, saved.RedirectProgress)
			}
		}
		if (p.DriveStatus == enum.DEVIATED || p.HasResume && p.ResumeDrive == enum.DEVIATED) && p.Redirect == nil {
			p.ResumeRedirect = true
		}
		t.progresses[id] = p
	}
}

func (t *TrackingCoordinator) restoreRedirect(p *engine.TravelProgress, saved model.RedirectCheckpoint, segment int, progress float64) {
	p.Redirect = &engine.RedirectState{
		Generation: saved.Generation, Reason: saved.Reason, Retries: saved.Retries,
		Hop: saved.Hop, Origin: saved.Origin, Target: saved.Target, Polyline: saved.Polyline,
		DistanceM: saved.DistanceM, DurationS: saved.DurationS, RequestedAt: saved.RequestedAt,
		RetryAt: saved.RetryAt, Failed: saved.Failed,
	}
	points, err := model.DecodePolyline(saved.Polyline)
	if err == nil && len(points) >= 2 {
		p.Redirect.Route = geo.NewRoute(points)
		p.RedirectSeg, p.RedirectProgress = segment, progress
	}
	p.ResumeRedirect = saved.Pending || p.Redirect.Route == nil
}

// Close runs on the room goroutine before the room is removed from the hub.
func (t *TrackingCoordinator) Close() {
	t.offerCheckpoint(time.Now(), true)
	for _, p := range t.progresses {
		p.StopSnap()
	}
	if t.unsubscribe != nil {
		t.unsubscribe()
	}
}

func (t *TrackingCoordinator) sendSnapshot(s realtime.Session, userID uuid.UUID) {
	snapshot := model.TripSnapshot{
		TripID: t.tripInformation.TripId.String(), Revision: t.revision,
		Status: t.tripInformation.Status, Riders: make(map[string]model.RiderState),
	}
	for id, p := range t.progresses {
		rider := model.RiderState{
			TrackingStatus: p.DriveStatus.String(), BranchID: p.BranchID,
			Lives: p.Lives, ImmuneUntil: p.ImmuneUntil, ConnectionStatus: p.ConnectionStatus,
			ObservedAt: p.ObservedAt, AwaitingFreshPosition: p.AwaitingFreshPosition,
		}
		if p.DriveStatus != enum.StayPrivate || id == userID {
			rider.HopIndex = p.HopIndex
			if !p.ObservedAt.IsZero() {
				lat, lng := p.Lat, p.Lng
				rider.Lat, rider.Lng = &lat, &lng
			}
			if p.Redirect != nil {
				checkpoint := t.captureRiderRedirect(p)
				rider.Redirect = checkpoint
			}
		}
		snapshot.Riders[id.String()] = rider
	}
	s.Send(userID, "tracking.snapshot", snapshot)
}

func (t *TrackingCoordinator) captureRiderRedirect(p *engine.TravelProgress) *model.RedirectCheckpoint {
	state := p.Redirect
	return &model.RedirectCheckpoint{
		Generation: state.Generation, Reason: state.Reason, Hop: state.Hop,
		Origin: state.Origin, Target: state.Target, Polyline: state.Polyline,
		DistanceM: state.DistanceM, DurationS: state.DurationS,
		Pending: state.Pending, Failed: state.Failed,
	}
}
