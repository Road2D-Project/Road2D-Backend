package message

type StayPublic bool

func (s StayPublic) GetType() TrackingMsgType {
	return UserStayPublic
}
