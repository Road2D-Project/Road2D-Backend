package chatting

import (
	"encoding/json"
	"time"

	"Road-To-Destination-BE/module/chatting/model/message"
	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func (c *ChattingCoordinator) OnMessage(s realtime.Session, userID uuid.UUID, command string, payload json.RawMessage) {
	state, online := c.members[userID]
	if !online {
		return
	}
	now := time.Now()
	switch command {
	case "resync":
		if !validResync(payload) {
			commandError(s, userID, "invalid_payload")
			return
		}
		if now.Sub(state.lastResync) < commandInterval {
			return
		}
		state.lastResync = now
		c.syncHint(s, userID)
	case "typing":
		var input message.TypingMessage
		if !decodeCommand(payload, &input) || input.Typing == nil {
			commandError(s, userID, "invalid_payload")
			return
		}
		if *input.Typing {
			if now.Sub(state.lastTyping) < commandInterval {
				return
			}
			state.typingUntil = now.Add(typingLifetime)
		} else {
			if !state.typingUntil.After(now) {
				return
			}
			state.typingUntil = time.Time{}
		}
		state.lastTyping = now
		c.sendTyping(s, userID, *input.Typing, now)
	default:
		commandError(s, userID, "unsupported_command")
		return
	}
	c.members[userID] = state
}

func (p *PersonalCoordinator) OnMessage(s realtime.Session, userID uuid.UUID, command string, payload json.RawMessage) {
	last, online := p.lastResync[userID]
	if !online {
		return
	}
	if command != "resync" {
		commandError(s, userID, "unsupported_command")
		return
	}
	if !validResync(payload) {
		commandError(s, userID, "invalid_payload")
		return
	}
	if time.Since(last) >= commandInterval {
		p.OnJoin(s, userID)
	}
}
