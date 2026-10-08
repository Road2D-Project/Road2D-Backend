package message

type TrackingMsgType string

const (
	Position TrackingMsgType = "position"
)

type TrackingMsg interface {
	GetType() TrackingMsgType
}
