package chatting

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func (c *ChattingCoordinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	c.members[userID] = memberState{lastResync: time.Now()}
	c.syncHint(s, userID)
}

func (p *PersonalCoordinator) OnJoin(s realtime.Session, userID uuid.UUID) {
	p.lastResync[userID] = time.Now()
	p.syncHint(s, userID)
}
