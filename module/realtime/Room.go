package realtime

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type inbound struct {
	client *Client
	env    Envelope
}

// Room is one realtime channel (a trip, later a chat). It owns connections,
// presence, fanout, and backpressure. Domain rules live in Module.
//
// Only Run touches clients, modules and domain state. Other goroutines submit
// work through channels; stopOnce only coordinates the shutdown signal.
type Room struct {
	id      string
	modules map[string]Module
	onClose func()
	session *roomSession
	options RoomOptions

	clients map[*Client]bool
	byUser  map[uuid.UUID]*Client // one live connection per user; the newest wins
	latest  map[string]map[string]any

	inbound    chan inbound
	register   chan *Client
	unregister chan *Client
	control    chan roomCommand
	done       chan struct{} // closed when the room dies, so pumps do not block
	stopping   chan struct{} // closes before cleanup; no new work should be admitted
	stopOnce   sync.Once
}

func NewRoom(id string, modules map[string]Module, onClose func()) *Room {
	return newRoom(id, modules, onClose, DefaultRoomOptions())
}

func newRoom(id string, modules map[string]Module, onClose func(), options RoomOptions) *Room {
	r := &Room{
		id:         id,
		modules:    modules,
		onClose:    onClose,
		options:    options,
		clients:    make(map[*Client]bool),
		byUser:     make(map[uuid.UUID]*Client),
		latest:     make(map[string]map[string]any),
		inbound:    make(chan inbound, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		control:    make(chan roomCommand, options.ControlBufferSize),
		done:       make(chan struct{}),
		stopping:   make(chan struct{}),
	}
	r.session = &roomSession{room: r}
	return r
}

func (r *Room) Run() {
	defer r.finish()
	var tick <-chan time.Time
	if r.options.TickInterval > 0 {
		ticker := time.NewTicker(r.options.TickInterval)
		defer ticker.Stop()
		tick = ticker.C
	}
	// A room nobody joins still has to close itself.
	idle := time.NewTimer(r.options.IdleTimeout)
	defer idle.Stop()
	idleTick := idle.C

	for {
		// Give shutdown priority over a continuously busy queue.
		select {
		case <-r.stopping:
			return
		default:
		}
		select {
		case <-r.stopping:
			return
		case c := <-r.register:
			r.onJoin(c)

		case c := <-r.unregister:
			r.onLeave(c)

		case in := <-r.inbound:
			r.handle(in)

		case command := <-r.control:
			r.handleCommand(command)

		case now := <-tick:
			r.tick(now)

		case <-idleTick:
			if len(r.clients) == 0 {
				return
			}
		}
		// Publication never refreshes an armed idle timer. Starting/stopping the
		// timer depends only on connection transitions, including eviction/drop.
		if len(r.clients) > 0 && idleTick != nil {
			idle.Stop()
			idleTick = nil
		} else if len(r.clients) == 0 && idleTick == nil {
			idle.Reset(r.options.IdleTimeout)
			idleTick = idle.C
		}
	}
}

func (r *Room) finish() {
	r.requestClose()
	for client := range r.clients {
		r.drop(client)
	}
	for _, module := range r.modules {
		if closer, ok := module.(interface{ Close() }); ok {
			closer.Close()
		}
	}
	clear(r.byUser)
	clear(r.latest)
	if r.onClose != nil {
		r.onClose()
	}
	close(r.done)
}

func (r *Room) close(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.requestClose()
	return r.waitClosed(ctx)
}

func (r *Room) requestClose() {
	r.stopOnce.Do(func() { close(r.stopping) })
}

func (r *Room) waitClosed(ctx context.Context) error {
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// join and submit select on done so the caller is not stuck after the room dies.
func (r *Room) join(c *Client) bool {
	select {
	case <-r.stopping:
		<-r.done
		return false
	default:
	}
	select {
	case r.register <- c:
		return true
	case <-r.done:
		return false
	case <-r.stopping:
		<-r.done
		return false
	}
}

func (r *Room) submit(in inbound) bool {
	select {
	case <-r.stopping:
		return false
	default:
	}
	select {
	case r.inbound <- in:
		return true
	case <-r.done:
		return false
	case <-r.stopping:
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

// drop is idempotent. Close the transport immediately so buffered frames cannot
// drain after an eviction ACK. The read pump posts unregister; normal slow-client
// OnLeave runs later, outside the module callback that caused the drop.
func (r *Room) drop(c *Client) {
	if r.clients[c] {
		delete(r.clients, c)
		close(c.send)
		if c.conn != nil {
			_ = c.conn.Close()
		}
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
	if r.options.TickInterval == 0 {
		// A tickless room has no batch consumer; retaining entries would grow
		// state that can never be delivered. Ordered server events still work.
		return
	}
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
