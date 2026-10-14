package message

type ChangeToStayMode bool

func (m ChangeToStayMode) GetType() TrackingMsgType {
	return StayMode
}
