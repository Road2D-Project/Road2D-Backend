package response

import (
	"encoding/json"
	"github.com/google/uuid"
	"time"
)

type ConversationEventResponse struct {
	EventID        uuid.UUID       `json:"eventId"`
	Type           string          `json:"type"`
	ConversationID uuid.UUID       `json:"conversationId"`
	EventSeq       int64           `json:"eventSeq"`
	Payload        json.RawMessage `json:"payload" swaggertype:"object"`
	CreatedTime    time.Time       `json:"createdTime"`
}
