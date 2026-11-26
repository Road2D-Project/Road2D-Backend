package outbox

import (
	"encoding/json"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RosterChanged       = "chat.rosterChanged"
	ConversationDeleted = "chat.conversationDeleted"
	SessionRevoked      = "auth.sessionRevoked"
)

// LifecyclePayload is internal integration metadata, never a replay payload.
type LifecyclePayload struct {
	ConversationID uuid.UUID   `json:"conversationId,omitempty"`
	RemovedUsers   []uuid.UUID `json:"removedUsers,omitempty"`
	SessionID      string      `json:"sessionId,omitempty"`
	BeforeVersion  int64       `json:"beforeVersion,omitempty"`
}

// AppendRosterChange must use the membership mutation's transaction, after
// locking the parent trip. Conversation locking matches message transactions.
func AppendRosterChange(tx *gorm.DB, tripID uuid.UUID, removed []uuid.UUID) error {
	var scope struct {
		ID       uuid.UUID
		EventSeq int64
	}
	result := tx.Raw(`UPDATE conversations SET next_event_seq = next_event_seq + 1
		WHERE trip_id = ? RETURNING id, next_event_seq - 1 AS event_seq`, tripID).Scan(&scope)
	if result.Error != nil || result.RowsAffected == 0 {
		return result.Error
	}
	payload, err := json.Marshal(LifecyclePayload{ConversationID: scope.ID, RemovedUsers: removed})
	if err != nil {
		return err
	}
	return tx.Create(&Event{Type: RosterChanged, AggregateID: scope.ID, ConversationID: &scope.ID, EventSeq: &scope.EventSeq, Payload: payload}).Error
}

func AppendSessionRevoked(tx *gorm.DB, userID uuid.UUID, sessionID string, beforeVersion int64) error {
	payload, err := json.Marshal(LifecyclePayload{SessionID: sessionID, BeforeVersion: beforeVersion})
	if err != nil {
		return err
	}
	return tx.Create(&Event{Type: SessionRevoked, AggregateID: userID, Payload: payload}).Error
}
