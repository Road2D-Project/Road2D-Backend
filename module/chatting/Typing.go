package chatting

import (
	"time"

	"Road-To-Destination-BE/module/chatting/model/message"
	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func (c *ChattingCoordinator) sendTyping(s realtime.Session, userID uuid.UUID, typing bool, now time.Time) {
	expires := now
	if typing {
		expires = now.Add(typingLifetime)
	}
	s.Broadcast("chat.typing", message.TypingEvent{
		ConversationID: c.id, UserID: userID, Typing: typing, ExpiresAt: expires.UTC(),
	})
}
