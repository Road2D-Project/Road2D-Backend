package chatting

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func (c *ChattingCoordinator) OnLeave(s realtime.Session, userID uuid.UUID) {
	if c.members[userID].typingUntil.After(time.Now()) {
		c.sendTyping(s, userID, false, time.Now())
	}
	delete(c.members, userID)
}

func (p *PersonalCoordinator) OnLeave(_ realtime.Session, userID uuid.UUID) {
	delete(p.lastResync, userID)
}
