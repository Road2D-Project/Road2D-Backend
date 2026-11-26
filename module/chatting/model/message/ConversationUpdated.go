package message

import (
	"time"

	"github.com/google/uuid"
)

// ConversationUpdated carries absolute personal state, never message content.
type ConversationUpdated struct {
	EventID             uuid.UUID  `json:"eventId"`
	ConversationID      uuid.UUID  `json:"conversationId"`
	SummaryVersion      int64      `json:"summaryVersion"`
	UnreadCount         int64      `json:"unreadCount"`
	LastReadSeq         int64      `json:"lastReadSeq"`
	LastMessageSeq      int64      `json:"lastMessageSeq"`
	LastMessageID       *uuid.UUID `json:"lastMessageId,omitempty"`
	LastMessageRevision *int64     `json:"lastMessageRevision,omitempty"`
	LastActivityAt      time.Time  `json:"lastActivityAt"`
	ActiveMemberCount   int64      `json:"activeMemberCount"`
	MutedUntil          *time.Time `json:"mutedUntil,omitempty"`
	Reason              string     `json:"reason"`
}
