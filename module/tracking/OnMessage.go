package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model/message"
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
	default:
		log.Printf("Unknown message type %s by user %s ", msgType, userID)
		return
	}
}
