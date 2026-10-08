package message

import "github.com/google/uuid"

type PositionMessage struct {
	Lat, Lng float64
	UserId   uuid.UUID
}

func (p PositionMessage) GetType() TrackingMsgType {
	return Position
}
