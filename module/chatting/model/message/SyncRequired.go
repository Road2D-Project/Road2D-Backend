package message

import "github.com/google/uuid"

// SyncRequired asks the client to read authoritative REST state. It is not a replay
// watermark and must never advance a client's durable event cursor by itself.
type SyncRequired struct {
	Scope          string     `json:"scope"`
	ConversationID *uuid.UUID `json:"conversationId,omitempty"`
}
