package chatting

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
)

// Chat hubs disable periodic ticks; typing expiry is carried in client events.
func (*ChattingCoordinator) OnTick(realtime.Session, time.Time) {}

func (*PersonalCoordinator) OnTick(realtime.Session, time.Time) {}
