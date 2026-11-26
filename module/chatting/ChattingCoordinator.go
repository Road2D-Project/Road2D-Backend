package chatting

import (
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

type memberState struct {
	lastTyping  time.Time
	typingUntil time.Time
	lastResync  time.Time
}

// ChattingCoordinator owns transient command state for one conversation room.
// Persistence and summaries stay in conversation services outside callbacks.
type ChattingCoordinator struct {
	id      uuid.UUID
	members map[uuid.UUID]memberState
}

// NewChattingModule builds one coordinator per conversation, without database IO.
func NewChattingModule() realtime.ModuleFactory {
	return func(roomID string) (realtime.Module, error) {
		id, err := uuid.Parse(roomID)
		if err != nil || id == uuid.Nil {
			return nil, realtime.ErrInvalidRoom
		}
		return &ChattingCoordinator{id: id, members: make(map[uuid.UUID]memberState)}, nil
	}
}
