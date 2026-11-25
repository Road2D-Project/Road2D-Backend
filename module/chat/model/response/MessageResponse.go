package response

import (
	"time"

	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
)

// MessageResponse excludes retry hashes, client operation IDs, and GORM relations.
type MessageResponse struct {
	ID               uuid.UUID        `json:"id"`
	ConversationID   uuid.UUID        `json:"conversationId"`
	SenderID         *uuid.UUID       `json:"senderId,omitempty"`
	Seq              int64            `json:"seq"`
	Kind             enum.MessageKind `json:"kind" swaggertype:"string"`
	Text             *string          `json:"text,omitempty"`
	ReplyToMessageID *uuid.UUID       `json:"replyToMessageId,omitempty"`
	Revision         int64            `json:"revision"`
	CreatedTime      time.Time        `json:"createdTime"`
	EditedAt         *time.Time       `json:"editedAt,omitempty"`
	DeletedAt        *time.Time       `json:"deletedAt,omitempty"`
}
