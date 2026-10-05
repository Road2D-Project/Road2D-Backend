package realtime

import "github.com/google/uuid"

// roomSession is the Session implementation bound to one room.
// Every method runs on the Run goroutine and touches room state directly.
type roomSession struct {
	room *Room
}

func (s *roomSession) Send(userID uuid.UUID, msgType string, payload any) {
	c, ok := s.room.byUser[userID]
	if !ok {
		return
	}
	s.room.sendTo(c, encode(msgType, payload))
}

func (s *roomSession) Broadcast(msgType string, payload any) {
	s.room.broadcast(encode(msgType, payload))
}

func (s *roomSession) BroadcastLatest(key string, msgType string, payload any) {
	s.room.setLatest(key, msgType, payload)
}

func (s *roomSession) IsOnline(userID uuid.UUID) bool {
	_, ok := s.room.byUser[userID]
	return ok
}
