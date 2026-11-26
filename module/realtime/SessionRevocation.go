package realtime

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionFilter prevents a delayed revocation from evicting a replacement
// connection authenticated under a newer session/version.
type SessionFilter struct {
	SessionID     string
	BeforeVersion int64
}

func (f SessionFilter) matches(c *Client) bool {
	return f.SessionID != "" && f.SessionID == c.options.SessionID ||
		f.BeforeVersion > 0 && c.options.SessionVersion < f.BeforeVersion
}

func (h *Hub) EvictSession(ctx context.Context, userID uuid.UUID, filter SessionFilter) error {
	if userID == uuid.Nil || filter.SessionID == "" && filter.BeforeVersion <= 0 {
		return ErrInvalidUser
	}
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, room := range h.rooms {
		rooms = append(rooms, room)
	}
	h.mu.Unlock()
	for i, room := range rooms {
		ack := make(chan struct{})
		accepted, err := room.enqueue(ctx, roomCommand{evictUser: userID, filter: &filter, ack: ack})
		if err != nil {
			return closeRevocationRooms(ctx, rooms[i:])
		}
		if !accepted {
			if err := room.waitClosed(ctx); err != nil {
				return closeRevocationRooms(ctx, rooms[i:])
			}
			continue
		}
		select {
		case <-ack:
		case <-room.done:
		case <-ctx.Done():
			return closeRevocationRooms(ctx, rooms[i:])
		}
	}
	return nil
}

// If the command queue or its ACK fails, request closure before waiting with a
// fresh bounded context. Reusing an expired request context would skip closure.
// Retained users can reconnect to newly created rooms after this fallback.
func closeRevocationRooms(ctx context.Context, rooms []*Room) error {
	force, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	for _, room := range rooms {
		room.requestClose()
	}
	for _, room := range rooms {
		if err := room.waitClosed(force); err != nil {
			return err
		}
	}
	return nil
}

func (h *Hub) HasRoom(ctx context.Context, id string) (bool, error) {
	room, err := h.existingRoom(ctx, id)
	if err != nil || room == nil {
		return false, err
	}
	select {
	case <-room.stopping:
		return false, nil
	default:
		return true, nil
	}
}
