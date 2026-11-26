package realtime

import (
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	pongWait        = 45 * time.Second
	pingPeriod      = 20 * time.Second
	writeWait       = 5 * time.Second
	maxMessageBytes = 4096
	sendBufferSize  = 64
)

// Client is one user's WebSocket connection inside one room.
// userID comes from auth. The room never trusts an id inside a client message.
type Client struct {
	room    *Room
	conn    *websocket.Conn
	send    chan []byte
	userID  uuid.UUID
	options ConnectionOptions
}

func newClient(room *Room, conn *websocket.Conn, userID uuid.UUID) *Client {
	return &Client{
		room:   room,
		conn:   conn,
		send:   make(chan []byte, sendBufferSize),
		userID: userID,
	}
}

func (c *Client) readPump() {
	defer func() {
		// Unregister through the room goroutine. If the room is already dead,
		// done is closed and this send must not block the pump forever.
		select {
		case c.room.unregister <- c:
		case <-c.room.done:
		}
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageBytes)
	_ = c.conn.SetReadDeadline(c.deadline(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(c.deadline(pongWait))
	})

	for {
		var env Envelope
		if err := c.conn.ReadJSON(&env); err != nil {
			return
		}
		if c.expired(time.Now()) {
			return
		}
		if !c.room.submit(inbound{client: c, env: env}) {
			return
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	var expiry <-chan time.Time
	if !c.options.ExpiresAt.IsZero() {
		timer := time.NewTimer(time.Until(c.options.ExpiresAt))
		defer timer.Stop()
		expiry = timer.C
	}

	for {
		if c.expired(time.Now()) {
			return
		}
		select {
		case <-expiry:
			return
		case msg, ok := <-c.send:
			if !ok {
				// The room dropped this client and closed send.
				return
			}
			if c.expired(time.Now()) {
				return
			}
			_ = c.conn.SetWriteDeadline(c.deadline(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(c.deadline(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) expired(now time.Time) bool {
	return !c.options.ExpiresAt.IsZero() && !c.options.ExpiresAt.After(now)
}

func (c *Client) deadline(wait time.Duration) time.Time {
	deadline := time.Now().Add(wait)
	if !c.options.ExpiresAt.IsZero() && c.options.ExpiresAt.Before(deadline) {
		return c.options.ExpiresAt
	}
	return deadline
}
