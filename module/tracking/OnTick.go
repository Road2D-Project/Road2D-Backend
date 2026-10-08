package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/utils/enum"
	"time"

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
			msg[i] = position{
				UserId:           userId,
				Lat:              progress.Lat,
				Lng:              progress.Lng,
				DriveStatus:      progress.DriveStatus,
				ConnectionStatus: progress.ConnectionStatus,
			}
			i++
		}
		s.Broadcast("tracking.positions", msg)
		i = 0
	}

}
