package message

type TrackingMsgType string

const (
	Position       TrackingMsgType = "position"
	StayMode       TrackingMsgType = "stayMode"
	UserStayPublic TrackingMsgType = "userStayPublic"
)

type TrackingMsg interface {
	GetType() TrackingMsgType
}
