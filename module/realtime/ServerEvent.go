package realtime

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
)

// ServerEvent is a prepared outbound frame, not a client command or a durable
// record. The caller authorizes and builds it outside the room. A zero UUID
// recipient broadcasts to the room; otherwise only that user's socket receives it.
type ServerEvent struct {
	Type            string
	Payload         json.RawMessage
	RecipientUserID uuid.UUID
}

type roomCommand struct {
	frame     []byte
	recipient uuid.UUID
	evictUser uuid.UUID
	ack       chan struct{}
	filter    *SessionFilter
}

// PublishToRoom enqueues a prepared event without creating a room. False means
// the room is absent/closing; true means queued, not received by any socket.
// A full queue fails immediately so a durable dispatcher can retry. Payload is
// serialized here, so the queued frame does not retain the caller's byte slice.
func (h *Hub) PublishToRoom(ctx context.Context, roomID string, event ServerEvent) (bool, error) {
	room, err := h.existingRoom(ctx, roomID)
	if err != nil || room == nil {
		return false, err
	}
	namespace, command, ok := strings.Cut(event.Type, ".")
	if !ok || namespace == "" || namespace == systemNamespace || command == "" {
		return false, ErrInvalidServerEvent
	}
	if len(event.Type)+len(event.Payload) > h.options.MaxEventBytes {
		return false, ErrEventTooLarge
	}
	frame, err := json.Marshal(Envelope{Type: event.Type, Payload: event.Payload})
	if err != nil {
		return false, ErrInvalidServerEvent
	}
	if len(frame) > h.options.MaxEventBytes {
		return false, ErrEventTooLarge
	}
	return room.enqueue(ctx, roomCommand{frame: frame, recipient: event.RecipientUserID})
}

// EvictUser waits until the room has removed the user and closed its socket, or
// until the whole room has finished closing. An absent user is an acknowledged
// no-op. A context error after enqueue does not retract the command; retry is safe.
// This does not prevent a later join; admission authorization belongs to callers.
func (h *Hub) EvictUser(ctx context.Context, roomID string, userID uuid.UUID) (bool, error) {
	if userID == uuid.Nil {
		return false, ErrInvalidUser
	}
	room, err := h.existingRoom(ctx, roomID)
	if err != nil || room == nil {
		return false, err
	}
	ack := make(chan struct{})
	accepted, err := room.enqueue(ctx, roomCommand{evictUser: userID, ack: ack})
	if err != nil {
		return false, err
	}
	if !accepted {
		// Closing is not an eviction ACK until sockets have actually been dropped.
		return true, room.waitClosed(ctx)
	}
	select {
	case <-ack:
		return true, nil
	case <-room.done:
		return true, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// CloseRoom closes only the currently stored room instance and waits for cleanup.
// A missing room is a no-op. A future authorized join may create another instance.
func (h *Hub) CloseRoom(ctx context.Context, roomID string) (bool, error) {
	room, err := h.existingRoom(ctx, roomID)
	if err != nil || room == nil {
		return false, err
	}
	return true, room.close(ctx)
}

func (h *Hub) existingRoom(ctx context.Context, roomID string) (*Room, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if roomID == "" || len(roomID) > maxRoomIDLen {
		return nil, ErrInvalidRoom
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return nil, ErrRoomClosed
	}
	return h.rooms[roomID], nil
}

func (r *Room) enqueue(ctx context.Context, command roomCommand) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	select {
	case <-r.stopping:
		return false, nil
	default:
	}
	select {
	case <-r.stopping:
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	case r.control <- command:
		return true, nil
	default:
		return false, ErrRoomQueueFull
	}
}

func (r *Room) handleCommand(command roomCommand) {
	if command.ack != nil {
		if client := r.byUser[command.evictUser]; client != nil && (command.filter == nil || command.filter.matches(client)) {
			r.onLeave(client)
		}
		close(command.ack)
		return
	}
	if command.recipient == uuid.Nil {
		r.broadcast(command.frame)
	} else if client := r.byUser[command.recipient]; client != nil {
		r.sendTo(client, command.frame)
	}
}
