package model

import "github.com/google/uuid"

const (
	EventMessageCreated     = "chat.messageCreated"
	EventMessageUpdated     = "chat.messageUpdated"
	EventMessageDeleted     = "chat.messageDeleted"
	EventReadUpdated        = "chat.readUpdated"
	EventPreferencesUpdated = "chat.preferencesUpdated"
)

// ConversationEventPayload is an invalidation hint, never message content.
type ConversationEventPayload struct {
	MessageID  *uuid.UUID `json:"messageId,omitempty"`
	MessageSeq *int64     `json:"messageSeq,omitempty"`
	Revision   *int64     `json:"revision,omitempty"`
	UserID     *uuid.UUID `json:"userId,omitempty"`
}
