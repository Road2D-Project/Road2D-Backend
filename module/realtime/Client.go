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
	room   *Room
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
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
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		var env Envelope
		if err := c.conn.ReadJSON(&env); err != nil {
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

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// The room dropped this client and closed send.
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
