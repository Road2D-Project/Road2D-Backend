package chatting

import (
	"Road-To-Destination-BE/module/chatting/model/message"
	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func (c *ChattingCoordinator) syncHint(s realtime.Session, userID uuid.UUID) {
	s.Send(userID, "chat.syncRequired", message.SyncRequired{Scope: "conversation", ConversationID: &c.id})
}

func (p *PersonalCoordinator) syncHint(s realtime.Session, userID uuid.UUID) {
	s.Send(userID, "chat.syncRequired", message.SyncRequired{Scope: "personal"})
}
