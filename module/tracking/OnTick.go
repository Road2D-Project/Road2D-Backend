package tracking

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnTick(s realtime.Session, now time.Time) {
	t.updateMemberPosition(s, now)()
	t.markTripComplete()
}
func (t *TrackingCoordinator) updateMemberPosition(s realtime.Session, now time.Time) func() {
	var (
		i = 0
		n = len(t.progresses)
	)
	type position struct {
		UserId           uuid.UUID
		Lat, Lng         float64
		DriveStatus      enum.DriveStatus
		ConnectionStatus enum.ConnectionStatus
	}
	msg := make([]position, n)
	return func() {
		status := t.tripInformation.Status
		if status == enum.TripCompleted {
			return
		}
		for userId, progress := range t.progresses {
			switch status {
			case enum.TripStay:
				if progress.DriveStatus != enum.StayPublic {
					continue
				}
			case enum.TripActive:
				if progress.DriveStatus == enum.STOP || progress.DriveStatus == enum.StayPrivate || progress.DriveStatus == enum.COMPLETED {
					continue
				}
				progress.ApplySnap()
				// A stopped rider is omitted. The client keeps the last pin it saw.
				if progress.DriveStatus != enum.COMPLETED {
					progress.SubmitSample()
				}
			default:
				continue
			}

			msg[i] = position{
				UserId:           userId,
				Lat:              progress.Lat,
				Lng:              progress.Lng,
				DriveStatus:      progress.DriveStatus,
				ConnectionStatus: progress.ConnectionStatus,
			}
			i++
		}
		if i == 0 {
			return
		}
		s.Broadcast("tracking.positions", msg[:i])
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
	t.tripInformation.Status = enum.TripCompleted
}
