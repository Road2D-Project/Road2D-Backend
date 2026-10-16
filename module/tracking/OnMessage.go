package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model/message"
	"Road-To-Destination-BE/utils/enum"
	"encoding/json"
	"log"
	"time"

	"github.com/google/uuid"
)

func (t *TrackingCoordinator) OnMessage(s realtime.Session, userID uuid.UUID, msgType string, payload json.RawMessage) {
	switch message.TrackingMsgType(msgType) {
	case message.Position:
		var positionMsg message.PositionMessage
		err := json.Unmarshal(payload, &positionMsg)
		if err != nil {
			return
		}
		progress := t.progresses[userID]
		if progress == nil {
			return
		}
		progress.Lat = positionMsg.Lat
		progress.Lng = positionMsg.Lng
		progress.ObservedAt = time.Now()
	case message.StayMode:
		rider := t.tripInformation.Riders[userID]
		if rider == nil || rider.Role != enum.TripRoleLeader {
			return
		}
		var stayMsg message.ChangeToStayMode
		err := json.Unmarshal(payload, &stayMsg)
		if err != nil {
			return
		}
		wantStay := bool(stayMsg)
		if t.tripInformation.Status == enum.TripActive && wantStay {
			t.beginStay()
			s.Broadcast(string(message.StayMode), map[string]any{
				"isChangeMode": true,
				"message":      "Leader change trip to stay mode. Let take a rest",
			})
			return
		}
		if t.tripInformation.IsStayMode() && !wantStay {
			t.endStay()
			s.Broadcast(string(message.StayMode), map[string]any{
				"isChangeMode": false,
				"message":      "Let go again",
			})
		}
		return
	case message.UserStayPublic:
		if !t.tripInformation.IsStayMode() {
			return
		}
		progress := t.progresses[userID]
		if progress == nil || progress.DriveStatus == enum.COMPLETED {
			return
		}
		var stayVisibility message.StayPublic
		err := json.Unmarshal(payload, &stayVisibility)
		if err != nil {
			return
		}
		if bool(stayVisibility) {
			progress.DriveStatus = enum.StayPublic
			return
		}
		progress.DriveStatus = enum.StayPrivate
		return

	default:
		log.Printf("Unknown message type %s by user %s ", msgType, userID)
		return
	}
}

// beginStay hides every rider who has not finished. A rider appears again only after choosing public.
func (t *TrackingCoordinator) beginStay() {
	t.tripInformation.Status = enum.TripStay
	for _, progress := range t.progresses {
		if progress.DriveStatus == enum.COMPLETED {
			continue
		}
		progress.DriveStatus = enum.StayPublic
	}
}

// endStay resumes the riders who were resting. A finished rider stays finished.
func (t *TrackingCoordinator) endStay() {
	t.tripInformation.Status = enum.TripActive
	for _, progress := range t.progresses {
		if progress.DriveStatus != enum.StayPublic && progress.DriveStatus != enum.StayPrivate {
			continue
		}
		progress.DriveStatus = enum.OnROUTE
	}
}
