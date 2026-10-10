package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/utils/enum"
	"log"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	rider := t.tripInformation.Riders[userID]
	if rider == nil {
		log.Printf("rider %s is not on this trip", userID)
		return
	}
	if prg, hasProgress := t.progresses[userID]; hasProgress {
		prg.ConnectionStatus = enum.CONNECTING
		s.Send(userID, "tracking.success", "Welcome back")
		return
	}
	t.progresses[userID] = engine.NewTravelProgress(userID, rider.AssignedBranchID)
	go engine.PendingPosition(t.progresses[userID], t.tripInformation)
	s.Send(userID, "tracking.success", "Go go")
}
