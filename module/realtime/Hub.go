package realtime

import (
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
}

func NewHub() *Hub {
	return &Hub{
		factories: make(map[string]ModuleFactory),
		rooms:     make(map[string]*Room),
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
	if roomID == "" || len(roomID) > maxRoomIDLen || userID == uuid.Nil || conn == nil {
		return ErrInvalidRoom
	}
	// Two attempts: the room can go idle between lookup and join.
	for attempt := 0; attempt < 2; attempt++ {
		room := h.getOrCreate(roomID)
		client := newClient(room, conn, userID)
		if room.join(client) {
			go client.writePump()
			go client.readPump()
			return nil
		}
	}
	return ErrRoomClosed
}

func (h *Hub) getOrCreate(roomID string) *Room {
	h.mu.Lock()
	if room, ok := h.rooms[roomID]; ok {
		h.mu.Unlock()
		return room
	}
	factories := make(map[string]ModuleFactory, len(h.factories))
	for namespace, factory := range h.factories {
		factories[namespace] = factory
	}
	h.mu.Unlock()

	// Factories run outside the lock so a slow constructor cannot stall every room.
	modules := make(map[string]Module, len(factories))
	for namespace, factory := range factories {
		modules[namespace] = factory(roomID)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if room, ok := h.rooms[roomID]; ok {
		// Another join created the room first. Drop the unused modules.
		return room
	}

	var room *Room
	room = NewRoom(roomID, modules, func() { h.remove(roomID, room) })
	h.rooms[roomID] = room
	go room.Run()
	return room
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
