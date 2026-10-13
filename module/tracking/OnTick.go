package tracking

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnTick(s realtime.Session, now time.Time) {
	t.updateMemberPosition(s, now)()
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
		for userId, progress := range t.progresses {
			progress.ApplySnap()
			// A stopped rider is omitted. The client keeps the last pin it saw.
			if progress.DriveStatus == enum.STOP {
				continue
			}
			progress.SubmitSample()
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
