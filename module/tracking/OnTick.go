package tracking

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"
)

func (t *TrackingCoordinator) OnTick(s realtime.Session, now time.Time) {
	t.drainRemoteRedirectResults()
	t.applyRedirectResults(s, now)
	t.advanceRedirectJobs(s, now)
	t.updateMemberPosition(s, now)()
	t.markTripComplete()
	t.offerCheckpoint(now, false)
}
func (t *TrackingCoordinator) updateMemberPosition(s realtime.Session, now time.Time) func() {
	var (
		i = 0
		n = len(t.progresses)
	)
	msg := make([]message.RiderPosition, n)
	return func() {
		status := t.tripInformation.Status
		if status == enum.TripCompleted {
			return
		}
		for userId, progress := range t.progresses {
			if progress.AwaitingFreshPosition {
				continue
			}
			switch status {
			case enum.TripStay:
				if progress.DriveStatus != enum.StayPublic {
					continue
				}
			case enum.TripActive:
				if progress.DriveStatus == enum.STOP || progress.DriveStatus == enum.StayPrivate || progress.DriveStatus == enum.COMPLETED {
					continue
				}
				if progress.ApplySnap() {
					t.markCheckpointDirty()
				}
				t.applyDeviation(s, progress, now)
				// A stopped rider is omitted. The client keeps the last pin it saw.
				if progress.DriveStatus != enum.COMPLETED {
					progress.SubmitSample()
				}
			default:
				continue
			}

			msg[i] = message.RiderPosition{
				UserID:           userId,
				Lat:              progress.Lat,
				Lng:              progress.Lng,
				BatteryPercent:   progress.BatteryPercent,
				SignalBars:       progress.SignalBars,
				DriveStatus:      progress.DriveStatus,
				ConnectionStatus: progress.ConnectionStatus,
			}
			i++
		}
		if i == 0 {
			return
		}
		s.Broadcast(message.OutPositions, msg[:i])
		i = 0
	}

}

// markTripComplete locks the trip once every rider who has joined this room has finished.
func (t *TrackingCoordinator) markTripComplete() {
	if t.tripInformation == nil || len(t.progresses) == 0 {
		return
	}
	for _, progress := range t.progresses {
		if progress.DriveStatus != enum.COMPLETED {
			return
		}
	}
	if t.tripInformation.Status != enum.TripCompleted {
		t.tripInformation.Status = enum.TripCompleted
		t.markCheckpointDirty()
		t.offerCheckpoint(time.Now(), true)
	}
}
