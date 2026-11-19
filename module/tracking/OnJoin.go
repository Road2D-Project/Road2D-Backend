package tracking

import (
	"log"

	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	rider := t.tripInformation.Riders[userID]
	if rider == nil {
		log.Printf("rider %s is not on this trip", userID)
		return
	}
	if t.redirectResults == nil && t.recovery.Results != nil {
		t.redirectResults, t.unsubscribe = t.recovery.Results.Subscribe(t.tripInformation.TripId)
	}
	if prg, hasProgress := t.progresses[userID]; hasProgress {
		prg.ConnectionStatus = enum.RECONNECTING
		s.Broadcast("tracking.connection", map[string]any{
			"userId":           userID,
			"connectionStatus": prg.ConnectionStatus,
		})
		prg.ConnectionStatus = enum.CONNECTING
		if prg.DriveStatus != enum.COMPLETED {
			prg.StartSnap(t.branch(prg.BranchID))
		}
		t.markCheckpointDirty()
		s.Send(userID, "tracking.success", "Welcome back")
		t.sendSnapshot(s, userID)
		return
	}
	t.progresses[userID] = engine.NewTravelProgress(userID, rider.AssignedBranchID)
	t.progresses[userID].StartSnap(t.branch(rider.AssignedBranchID))
	s.Send(userID, "tracking.success", "Go go")
	t.markCheckpointDirty()
	t.sendSnapshot(s, userID)
}

func (t *TrackingCoordinator) branch(id uuid.UUID) *model.TravelBranch {
	if t.tripInformation == nil {
		return nil
	}
	return t.tripInformation.Branches[id]
}
