package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/enum"
	"time"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	rider, _ := t.tripInformation.Riders[userID]
	if prg, hasProgress := t.progresses[userID]; hasProgress {
		prg.ConnectionStatus = enum.CONNECTING
		s.Send(userID, "tracking.success", "Welcome back")
		return
	}
	riderProgress := &model.TravelProgress{
		UserID:           userID,
		BranchID:         rider.AssignedBranchID,
		HopIndex:         0,
		DriveStatus:      enum.OnROUTE,
		Lat:              0,
		Lng:              0,
		ObservedAt:       time.Time{},
		OffRouteSamples:  0,
		ArriveSamples:    0,
		Lives:            0,
		ImmuneUntil:      time.Time{},
		Arrived:          nil,
		ConnectionStatus: enum.CONNECTING,
	}
	t.progresses[userID] = riderProgress
	s.Send(userID, "tracking.success", "Go go")
}
