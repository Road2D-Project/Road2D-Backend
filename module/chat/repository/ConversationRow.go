package repository

import (
	"time"

	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
)

// ConversationRow is a per-viewer projection from a single PostgreSQL snapshot.
type ConversationRow struct {
	ID, TripID          uuid.UUID
	Name                string
	MyRole              enum.TripRole
	ActiveMemberCount   int64
	UnreadCount         int64
	LastReadSeq         int64
	LastMessageSeq      int64
	LastMessageID       *uuid.UUID
	LastMessageRevision *int64
	LastMessageKind     enum.MessageKind
	LastMessageText     *string
	LastMessageDeleted  bool
	LastActivityAt      time.Time
	SummaryVersion      int64
	MutedUntil          *time.Time
}

type ConversationCursor struct {
	ActivityAt time.Time `json:"activityAt"`
	ID         uuid.UUID `json:"id"`
}
