package realtime

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const maxRoomIDLen = 128

// Hub owns the rooms and the module factories mounted into each new room.
type Hub struct {
	mu        sync.Mutex
	factories map[string]ModuleFactory
	rooms     map[string]*Room
	closing   bool
	options   RoomOptions
}

func NewHub() *Hub {
	return newHub(DefaultRoomOptions())
}

// NewHubWithOptions creates an independent runtime, for example a chat hub with
// a shorter idle timeout and no periodic ticks. Options are copied, not shared.
func NewHubWithOptions(options RoomOptions) (*Hub, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	return newHub(options), nil
}

func newHub(options RoomOptions) *Hub {
	return &Hub{
		factories: make(map[string]ModuleFactory),
		rooms:     make(map[string]*Room),
		options:   options,
	}
}

// Mount registers a module factory for a namespace. Call it before accepting
// connections: a room created earlier does not gain modules mounted later.
func (h *Hub) Mount(namespace string, factory ModuleFactory) error {
	if factory == nil || namespace == "" || namespace == systemNamespace || strings.Contains(namespace, ".") {
		return fmt.Errorf("%w: %q", ErrInvalidNamespace, namespace)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, exists := h.factories[namespace]; exists {
		return fmt.Errorf("%w: %q already mounted", ErrInvalidNamespace, namespace)
	}
	h.factories[namespace] = factory
	return nil
}

// Join attaches an already-upgraded, already-authenticated connection to a room
// and starts the two pumps. The room id is an opaque string (a trip UUID text
// for the lobby). The user id is the authenticated user, never a value from the socket.
func (h *Hub) Join(roomID string, userID uuid.UUID, conn *websocket.Conn) error {
	return h.join(roomID, userID, conn, ConnectionOptions{})
}

// JoinWithOptions attaches an authenticated expiring connection. Tracking's
// existing Join remains unchanged; new chat connections must use this method.
func (h *Hub) JoinWithOptions(roomID string, userID uuid.UUID, conn *websocket.Conn, options ConnectionOptions) error {
	if err := options.Validate(); err != nil {
		return err
	}
	return h.join(roomID, userID, conn, options)
}

func (h *Hub) join(roomID string, userID uuid.UUID, conn *websocket.Conn, options ConnectionOptions) error {
	if roomID == "" || len(roomID) > maxRoomIDLen || userID == uuid.Nil || conn == nil {
		return ErrInvalidRoom
	}
	// Two attempts: the room can go idle between lookup and join.
	for attempt := 0; attempt < 2; attempt++ {
		room, err := h.getOrCreate(roomID)
		if err != nil {
			return err
		}
		client := newClient(room, conn, userID)
		client.options = options
		if room.join(client) {
			go client.writePump()
			go client.readPump()
			return nil
		}
	}
	return ErrRoomClosed
}

func (h *Hub) getOrCreate(roomID string) (*Room, error) {
	h.mu.Lock()
	if h.closing {
		h.mu.Unlock()
		return nil, ErrRoomClosed
	}
	if room, ok := h.rooms[roomID]; ok {
		h.mu.Unlock()
		return room, nil
	}
	factories := make(map[string]ModuleFactory, len(h.factories))
	for namespace, factory := range h.factories {
		factories[namespace] = factory
	}
	h.mu.Unlock()

	// Factories run outside the lock so a slow constructor cannot stall every room.
	modules := make(map[string]Module, len(factories))
	for namespace, factory := range factories {
		module, err := factory(roomID)
		if err != nil {
			return nil, err
		}
		if module == nil {
			// A peer may have created the room while this factory failed.
			h.mu.Lock()
			room, ok := h.rooms[roomID]
			h.mu.Unlock()
			if ok {
				return room, nil
			}
			return nil, fmt.Errorf("%w: %s", ErrModuleUnavailable, namespace)
		}
		modules[namespace] = module
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closing {
		return nil, ErrRoomClosed
	}
	if room, ok := h.rooms[roomID]; ok {
		// Another join created the room first. Drop the unused modules.
		return room, nil
	}

	var room *Room
	room = newRoom(roomID, modules, func() { h.remove(roomID, room) }, h.options)
	h.rooms[roomID] = room
	go room.Run()
	return room, nil
}

// Close finishes module callbacks before persistence workers are drained.
func (h *Hub) Close(ctx context.Context) error {
	h.mu.Lock()
	h.closing = true
	rooms := make([]*Room, 0, len(h.rooms))
	for _, room := range h.rooms {
		rooms = append(rooms, room)
	}
	h.mu.Unlock()
	// Signal every room before waiting: one slow close hook must not leave the
	// other rooms accepting traffic when the shutdown deadline is exhausted.
	for _, room := range rooms {
		room.requestClose()
	}
	for _, room := range rooms {
		if err := room.waitClosed(ctx); err != nil {
			return err
		}
	}
	return nil
}

// remove deletes the room only when it is still the same pointer.
// A newer room with the same id must stay.
func (h *Hub) remove(roomID string, room *Room) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == room {
		delete(h.rooms, roomID)
	}
}
