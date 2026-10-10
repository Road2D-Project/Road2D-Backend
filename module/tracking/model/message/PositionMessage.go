package message

type PositionMessage struct {
	Lat, Lng float64
}

func (p PositionMessage) GetType() TrackingMsgType {
	return Position
}
