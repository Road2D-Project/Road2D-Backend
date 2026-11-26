package chatting

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

const (
	commandInterval = time.Second
	typingLifetime  = 5 * time.Second
)

func decodeCommand(payload json.RawMessage, target any) bool {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 || payload[0] != '{' {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return false
	}
	return decoder.Decode(new(any)) == io.EOF
}

func validResync(payload json.RawMessage) bool {
	return len(payload) == 0 || decodeCommand(payload, &struct{}{})
}

func commandError(s realtime.Session, userID uuid.UUID, code string) {
	s.Send(userID, realtime.MsgTypeError, struct {
		Code string `json:"code"`
	}{Code: code})
}
