package response

import (
	"time"

	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
)

type ConversationResponse struct {
	ID                  uuid.UUID       `json:"id"`
	TripID              uuid.UUID       `json:"tripId"`
	Name                string          `json:"name"`
	MyRole              enum.TripRole   `json:"myRole" swaggertype:"string"`
	ActiveMemberCount   int64           `json:"activeMemberCount"`
	UnreadCount         int64           `json:"unreadCount"`
	LastReadSeq         int64           `json:"lastReadSeq"`
	LastMessageSeq      int64           `json:"lastMessageSeq"`
	LastMessageID       *uuid.UUID      `json:"lastMessageId,omitempty"`
	LastMessageRevision *int64          `json:"lastMessageRevision,omitempty"`
	LastActivityAt      time.Time       `json:"lastActivityAt"`
	SummaryVersion      int64           `json:"summaryVersion"`
	MutedUntil          *time.Time      `json:"mutedUntil,omitempty"`
	LastMessagePreview  *MessagePreview `json:"lastMessagePreview,omitempty"`
}
