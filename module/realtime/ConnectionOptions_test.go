package realtime

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestExpiringConnectionClosesTransportWithoutWaitingForHeartbeat(t *testing.T) {
	hub := testRuntimeHub(t, time.Minute, 0)
	serverErrors := make(chan error, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		serverErrors <- hub.JoinWithOptions("expiring", uuid.New(), conn, ConnectionOptions{
			TokenID: uuid.NewString(), SessionVersion: 1, ExpiresAt: time.Now().Add(100 * time.Millisecond),
		})
	}))
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := <-serverErrors; err != nil {
		t.Fatal(err)
	}
	_ = readOutgoing(t, conn)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err = conn.ReadMessage()
	var timeout net.Error
	if err == nil || (errors.As(err, &timeout) && timeout.Timeout()) {
		t.Fatalf("expired socket remained open: %v", err)
	}
}

func TestExpiredOrMissingConnectionIdentityCannotCreateRoom(t *testing.T) {
	hub := NewHub()
	for _, options := range []ConnectionOptions{
		{},
		{TokenID: "token", SessionVersion: 1, ExpiresAt: time.Now().Add(-time.Second)},
		{TokenID: "token", ExpiresAt: time.Now().Add(time.Hour)},
	} {
		err := hub.JoinWithOptions("room", uuid.New(), &websocket.Conn{}, options)
		if !errors.Is(err, ErrInvalidConnectionOptions) && !errors.Is(err, ErrConnectionExpired) {
			t.Fatalf("invalid identity accepted: %v", err)
		}
	}
	if len(hub.rooms) != 0 {
		t.Fatal("invalid identity created a room")
	}
	_ = hub.Close(context.Background())
}
