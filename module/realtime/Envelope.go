package realtime

import (
	"encoding/json"
	"log"

	"github.com/google/uuid"
)

// systemNamespace belongs to the infra. A domain module must not Mount this name.
const systemNamespace = "system"

const (
	MsgTypePresence = systemNamespace + ".presence"
	MsgTypeError    = systemNamespace + ".error"

	// BroadcastLatest flushes with type = <msgType> + batchSuffix.
	batchSuffix = ".batch"

	errCodeInvalidType      = "invalid_type"
	errCodeUnknownNamespace = "unknown_namespace"
)

// Envelope is one client-to-server message. The room reads Type only to route
// on the namespace ("tracking.location" -> module "tracking"). The module
// unmarshals Payload itself.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// outgoing is one server-to-client message.
type outgoing struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

type presencePayload struct {
	UserID uuid.UUID `json:"userId"`
	Online bool      `json:"online"`
}

type errorPayload struct {
	Code string `json:"code"`
}

type latestEntry struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// encode returns nil when marshal fails. sendTo skips a nil frame so one bad
// payload cannot kill the room goroutine.
func encode(msgType string, payload any) []byte {
	b, err := json.Marshal(outgoing{Type: msgType, Payload: payload})
	if err != nil {
		log.Printf("realtime: marshal %s: %v", msgType, err)
		return nil
	}
	return b
}
