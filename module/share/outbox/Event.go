package outbox

import (
	chatModel "Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/utils"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Event is inserted with the domain mutation. Dispatch is not a client receipt.
// Independent cleanup jobs omit ConversationID so they survive conversation deletion.
type Event struct {
	utils.Base
	Type           string                  `json:"type" gorm:"type:varchar(64);not null"`
	AggregateID    uuid.UUID               `json:"-" gorm:"type:uuid;not null"`
	ConversationID *uuid.UUID              `json:"conversationId,omitempty" gorm:"type:uuid;uniqueIndex:idx_outbox_conversation_seq"`
	Conversation   *chatModel.Conversation `json:"-" gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// No SET NULL user FK: deleting a recipient must never make a private event public.
	RecipientUserID *uuid.UUID      `json:"-" gorm:"type:uuid"`
	EventSeq        *int64          `json:"eventSeq,omitempty" gorm:"uniqueIndex:idx_outbox_conversation_seq;check:chk_outbox_sequence,event_seq IS NULL OR (conversation_id IS NOT NULL AND event_seq > 0)"`
	Payload         json.RawMessage `json:"payload" gorm:"type:jsonb;not null"`
	Attempts        int             `json:"-" gorm:"not null;default:0;check:chk_outbox_attempts,attempts >= 0"`
	NextAttemptAt   time.Time       `json:"-" gorm:"not null;default:CURRENT_TIMESTAMP;index:idx_outbox_pending,priority:1,where:dispatched_at IS NULL"`
	DispatchedAt    *time.Time      `json:"-"`
}

func (Event) TableName() string { return "outbox_events" }
