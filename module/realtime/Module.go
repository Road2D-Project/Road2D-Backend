package realtime

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Module is the domain behavior attached to one room (tracking, chat, and so on).
//
// Room calls every method from the single Run goroutine, so a module does not
// need its own mutex. A module must not block (synchronous DB or HTTP): that
// stalls every connection in the room.
type Module interface {
	// OnJoin runs when the user is online, including a reconnect that replaces
	// the previous connection. It must be idempotent. This is where the module
	// sends that user its snapshot.
	OnJoin(s Session, userID uuid.UUID)

	// OnLeave runs only when the user is actually offline (the live connection dropped).
	// A connection that was replaced does not get OnLeave.
	OnLeave(s Session, userID uuid.UUID)

	// OnMessage receives a message addressed to this module. msgType has the
	// namespace prefix removed ("tracking.location" arrives as "location").
	OnMessage(s Session, userID uuid.UUID, msgType string, payload json.RawMessage)

	// OnTick runs every tickInterval so the module can advance time-based work.
	// The room may already be empty; it stays alive until roomIdleTTL.
	OnTick(s Session, now time.Time)
}

// ModuleFactory builds one module instance per room, so state is never shared
// across rooms. The factory must not call back into the Hub and must not start
// its own goroutines. A slow factory does not hold the hub lock, but it still
// delays the first join of that room.
type ModuleFactory func(roomID string) Module
