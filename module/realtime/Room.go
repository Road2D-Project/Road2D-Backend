package realtime

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	tickInterval = 500 * time.Millisecond // OnTick and BroadcastLatest flush
	roomIdleTTL  = 5 * time.Minute
)

type inbound struct {
	client *Client
	env    Envelope
}

// Room is one realtime channel (a trip, later a chat). It owns connections,
// presence, fanout, and backpressure. Domain rules live in Module.
//
// Only the Run goroutine may touch the fields below, so Room has no mutex.
type Room struct {
	id      string
	modules map[string]Module
	onClose func()
	session *roomSession

	clients map[*Client]bool
	byUser  map[uuid.UUID]*Client // one live connection per user; the newest wins
	latest  map[string]map[string]any

	inbound    chan inbound
	register   chan *Client
	unregister chan *Client
	done       chan struct{} // closed when the room dies, so pumps do not block
}

func NewRoom(id string, modules map[string]Module, onClose func()) *Room {
	r := &Room{
		id:         id,
		modules:    modules,
		onClose:    onClose,
		clients:    make(map[*Client]bool),
		byUser:     make(map[uuid.UUID]*Client),
		latest:     make(map[string]map[string]any),
		inbound:    make(chan inbound, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
	r.session = &roomSession{room: r}
	return r
}

func (r *Room) Run() {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()
	// A room nobody joins still has to close itself.
	idleSince := time.Now()

	for {
		select {
		case c := <-r.register:
			r.onJoin(c)
			idleSince = time.Time{}

		case c := <-r.unregister:
			r.onLeave(c)
			if len(r.byUser) == 0 && idleSince.IsZero() {
				idleSince = time.Now()
			}

		case in := <-r.inbound:
			r.handle(in)

		case now := <-ticker.C:
			r.tick(now)
			if len(r.byUser) == 0 && !idleSince.IsZero() && now.Sub(idleSince) > roomIdleTTL {
				r.onClose()
				close(r.done)
				return
			}
		}
	}
}

// join and submit select on done so the caller is not stuck after the room dies.
func (r *Room) join(c *Client) bool {
	select {
	case r.register <- c:
		return true
	case <-r.done:
		return false
	}
}

func (r *Room) submit(in inbound) bool {
	select {
	case r.inbound <- in:
		return true
	case <-r.done:
		return false
	}
}

func (r *Room) onJoin(c *Client) {
	old, reconnected := r.byUser[c.userID]
	if reconnected {
		// The new connection wins. The previous one is treated as already dead
		// (a half-open socket), so it is dropped without an offline presence.
		r.drop(old)
	}
	r.clients[c] = true
	r.byUser[c.userID] = c

	for _, m := range r.modules {
		m.OnJoin(r.session, c.userID)
	}
	if !reconnected {
		r.broadcast(encode(MsgTypePresence, presencePayload{UserID: c.userID, Online: true}))
	}
}

func (r *Room) onLeave(c *Client) {
	r.drop(c)
	if r.byUser[c.userID] != c {
		// This socket was already replaced. Clean it up, but the user is still online.
		return
	}
	delete(r.byUser, c.userID)

	for _, m := range r.modules {
		m.OnLeave(r.session, c.userID)
	}
	r.broadcast(encode(MsgTypePresence, presencePayload{UserID: c.userID, Online: false}))
}

// drop is idempotent: close(send) runs once. writePump sees the closed channel,
// closes the socket, and readPump then posts unregister. onLeave therefore
// always runs later, outside the module callback that caused the drop.
func (r *Room) drop(c *Client) {
	if r.clients[c] {
		delete(r.clients, c)
		close(c.send)
	}
}

// sendTo is the only writer of c.send. A full buffer means a slow consumer:
// drop that socket here, and do not call onLeave inline. An inline leave would
// re-enter the module from inside its own callback.
func (r *Room) sendTo(c *Client, b []byte) {
	if b == nil || !r.clients[c] {
		return
	}
	select {
	case c.send <- b:
	default:
		r.drop(c)
	}
}

func (r *Room) broadcast(b []byte) {
	for c := range r.clients {
		// Deleting the current map entry during range is safe in Go.
		r.sendTo(c, b)
	}
}

func (r *Room) handle(in inbound) {
	if r.byUser[in.client.userID] != in.client {
		// Frame from a socket that has already been replaced.
		return
	}
	namespace, msgType, ok := strings.Cut(in.env.Type, ".")
	if !ok || msgType == "" {
		r.sendError(in.client, errCodeInvalidType)
		return
	}
	m, ok := r.modules[namespace]
	if !ok {
		r.sendError(in.client, errCodeUnknownNamespace)
		return
	}
	m.OnMessage(r.session, in.client.userID, msgType, in.env.Payload)
}

func (r *Room) sendError(c *Client, code string) {
	r.sendTo(c, encode(MsgTypeError, errorPayload{Code: code}))
}

func (r *Room) tick(now time.Time) {
	for _, m := range r.modules {
		m.OnTick(r.session, now)
	}
	r.flushLatest()
}

func (r *Room) setLatest(key string, msgType string, payload any) {
	entries, ok := r.latest[msgType]
	if !ok {
		entries = make(map[string]any)
		r.latest[msgType] = entries
	}
	entries[key] = payload
}

// flushLatest keeps the newest value per key, marshals once per message type,
// and fans that batch out to the room.
func (r *Room) flushLatest() {
	for msgType, entries := range r.latest {
		if len(entries) == 0 {
			continue
		}
		batch := make([]latestEntry, 0, len(entries))
		for key, value := range entries {
			batch = append(batch, latestEntry{Key: key, Value: value})
		}
		clear(entries)
		r.broadcast(encode(msgType+batchSuffix, batch))
	}
}
