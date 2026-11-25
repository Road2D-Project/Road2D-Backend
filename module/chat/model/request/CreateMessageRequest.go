package request

import "github.com/google/uuid"

type CreateMessageRequest struct {
	ClientMessageID  uuid.UUID  `json:"clientMessageId" binding:"required" swaggertype:"string" format:"uuid"`
	Text             string     `json:"text" binding:"required,max=4000"`
	ReplyToMessageID *uuid.UUID `json:"replyToMessageId,omitempty"`
}
