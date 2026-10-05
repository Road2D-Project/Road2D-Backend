package realtime

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestMountRejectsReservedDuplicateAndEmpty(t *testing.T) {
	hub := NewHub()
	factory := func(string) Module { return nopModule{} }

	if err := hub.Mount("system", factory); err == nil || !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("system namespace: %v", err)
	}
	if err := hub.Mount("tracking.location", factory); err == nil {
		t.Fatal("dotted namespace was accepted")
	}
	if err := hub.Mount("", factory); err == nil {
		t.Fatal("empty namespace was accepted")
	}
	if err := hub.Mount("tracking", nil); err == nil {
		t.Fatal("nil factory was accepted")
	}
	if err := hub.Mount("tracking", factory); err != nil {
		t.Fatal(err)
	}
	if err := hub.Mount("tracking", factory); err == nil || !errors.Is(err, ErrInvalidNamespace) {
		t.Fatalf("duplicate: %v", err)
	}
}

func TestJoinRejectsBadIdentity(t *testing.T) {
	hub := NewHub()
	conn := &websocket.Conn{}
	if err := hub.Join("", uuid.New(), conn); err == nil {
		t.Fatal("empty room id was accepted")
	}
	if err := hub.Join("room", uuid.Nil, conn); err == nil {
		t.Fatal("nil user was accepted")
	}
	if err := hub.Join(strings.Repeat("a", maxRoomIDLen+1), uuid.New(), conn); err == nil {
		t.Fatal("overlong room id was accepted")
	}
}

func TestRoomRoutesMessageAndBatchesLatest(t *testing.T) {
	mod := newScriptModule()
	hub := NewHub()
	if err := hub.Mount("tracking", func(string) Module { return mod }); err != nil {
		t.Fatal(err)
	}

	userID := uuid.New()
	conn := dialRoom(t, hub, "trip-1", userID)
	defer conn.Close()

	presence := readOutgoing(t, conn)
	if presence.Type != MsgTypePresence {
		t.Fatalf("first frame %s", presence.Type)
	}

	writeEnvelope(t, conn, Envelope{Type: "tracking.ping", Payload: json.RawMessage(`{"n":1}`)})
	pong := readOutgoing(t, conn)
	if pong.Type != "tracking.pong" {
		t.Fatalf("pong type %s", pong.Type)
	}

	writeEnvelope(t, conn, Envelope{Type: "tracking.location", Payload: json.RawMessage(`{"lat":1}`)})
	batch := readOutgoing(t, conn)
	if batch.Type != "tracking.location.batch" {
		t.Fatalf("batch type %s", batch.Type)
	}
	raw, err := json.Marshal(batch.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var entries []latestEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Key != userID.String() {
		t.Fatalf("batch entries %#v", entries)
	}

	writeEnvelope(t, conn, Envelope{Type: "nope"})
	invalid := readOutgoing(t, conn)
	assertErrorCode(t, invalid, errCodeInvalidType)

	writeEnvelope(t, conn, Envelope{Type: "chat.hi"})
	unknown := readOutgoing(t, conn)
	assertErrorCode(t, unknown, errCodeUnknownNamespace)
}

func TestReplacedConnectionDoesNotLeave(t *testing.T) {
	mod := newScriptModule()
	hub := NewHub()
	if err := hub.Mount("tracking", func(string) Module { return mod }); err != nil {
		t.Fatal(err)
	}
	userID := uuid.New()

	first := dialRoom(t, hub, "trip-1", userID)
	mod.waitJoin(t)
	_ = readOutgoing(t, first)

	second := dialRoom(t, hub, "trip-1", userID)
	mod.waitJoin(t)
	defer second.Close()

	// The room drops the old socket. Its unregister must not count as offline.
	_ = first.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := first.ReadMessage(); err == nil {
		t.Fatal("replaced connection stayed open")
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if got := mod.leaveCount(); got != 0 {
			t.Fatalf("leaves after reconnect = %d", got)
		}
		time.Sleep(20 * time.Millisecond)
	}

	_ = first.Close()
	_ = second.Close()
	mod.waitLeave(t)
	if got := mod.leaveCount(); got != 1 {
		t.Fatalf("leaves after close = %d", got)
	}
}

type nopModule struct{}

func (nopModule) OnJoin(Session, uuid.UUID)                             {}
func (nopModule) OnLeave(Session, uuid.UUID)                            {}
func (nopModule) OnMessage(Session, uuid.UUID, string, json.RawMessage) {}
func (nopModule) OnTick(Session, time.Time)                             {}

type scriptModule struct {
	mu     sync.Mutex
	leaves int
	joined chan struct{}
	left   chan struct{}
}

func newScriptModule() *scriptModule {
	return &scriptModule{
		joined: make(chan struct{}, 4),
		left:   make(chan struct{}, 4),
	}
}

func (m *scriptModule) OnJoin(Session, uuid.UUID) {
	m.joined <- struct{}{}
}

func (m *scriptModule) OnLeave(Session, uuid.UUID) {
	m.mu.Lock()
	m.leaves++
	m.mu.Unlock()
	m.left <- struct{}{}
}

func (m *scriptModule) OnMessage(s Session, userID uuid.UUID, msgType string, payload json.RawMessage) {
	switch msgType {
	case "ping":
		s.Broadcast("tracking.pong", map[string]any{"userId": userID.String()})
	case "location":
		var body map[string]any
		if err := json.Unmarshal(payload, &body); err != nil {
			return
		}
		s.BroadcastLatest(userID.String(), "tracking.location", body)
	}
}

func (m *scriptModule) OnTick(Session, time.Time) {}

func (m *scriptModule) waitJoin(t *testing.T) {
	t.Helper()
	select {
	case <-m.joined:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for join")
	}
}

func (m *scriptModule) waitLeave(t *testing.T) {
	t.Helper()
	select {
	case <-m.left:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for leave")
	}
}

func (m *scriptModule) leaveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.leaves
}

func dialRoom(t *testing.T, hub *Hub, roomID string, userID uuid.UUID) *websocket.Conn {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		if err := hub.Join(r.URL.Query().Get("room"), userID, conn); err != nil {
			conn.Close()
		}
	}))
	t.Cleanup(srv.Close)

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "?room=" + roomID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func writeEnvelope(t *testing.T, conn *websocket.Conn, env Envelope) {
	t.Helper()
	if err := conn.WriteJSON(env); err != nil {
		t.Fatal(err)
	}
}

func readOutgoing(t *testing.T, conn *websocket.Conn) outgoing {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg outgoing
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read: %v", err)
	}
	return msg
}

func assertErrorCode(t *testing.T, msg outgoing, code string) {
	t.Helper()
	if msg.Type != MsgTypeError {
		t.Fatalf("type %s", msg.Type)
	}
	raw, err := json.Marshal(msg.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var body errorPayload
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code {
		t.Fatalf("code %s", body.Code)
	}
}
