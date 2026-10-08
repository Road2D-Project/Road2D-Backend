package tracking

import (
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/tracking/model/message"
	"encoding/json"
	"log"

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
		t.progresses[userID].Lat = positionMsg.Lat
		t.progresses[userID].Lng = positionMsg.Lng
	default:
		log.Print("Unknown message type: ", msgType)
		return
	}
}
