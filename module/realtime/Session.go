package realtime

import "github.com/google/uuid"

// Session is the only handle a domain module gets to the room.
// It is small so tests can fake it. Call it only from Module callbacks,
// which run on the same goroutine as Room.Run.
type Session interface {
	// Send writes to one user and does nothing when that user is offline.
	Send(userID uuid.UUID, msgType string, payload any)

	// Broadcast writes immediately to every connection in the room.
	// That path keeps order. Use it for state the client must not skip.
	Broadcast(msgType string, payload any)

	// BroadcastLatest keeps only the newest value for (msgType, key) and
	// flushes it once per tick as "<msgType>.batch". Use it for lossy streams
	// such as positions. key is an opaque id, usually the user id string.
	BroadcastLatest(key string, msgType string, payload any)

	IsOnline(userID uuid.UUID) bool
}
