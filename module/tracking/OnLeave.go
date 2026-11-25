package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnLeave(s realtime.Session, userID uuid.UUID) {
	progress := t.progresses[userID]
	if progress == nil {
		return
	}
	progress.ConnectionStatus = enum.DISCONNECTED
	progress.StopSnap()
	t.markCheckpointDirty()
	t.offerCheckpoint(time.Now(), true)
	s.Broadcast("tracking.onLeave", "Goodbye user"+userID.String())
}
