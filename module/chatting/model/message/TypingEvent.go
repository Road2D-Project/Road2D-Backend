package message

import (
	"time"

	"github.com/google/uuid"
)

type TypingEvent struct {
	ConversationID uuid.UUID `json:"conversationId"`
	UserID         uuid.UUID `json:"userId"`
	Typing         bool      `json:"typing"`
	ExpiresAt      time.Time `json:"expiresAt"`
}
