package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
)

func TestServerOperationsDoNotCreateOfflineRooms(t *testing.T) {
	hub := NewHub()
	ctx := context.Background()
	if accepted, err := hub.PublishToRoom(ctx, "offline", ServerEvent{Type: "chat.changed"}); accepted || err != nil {
		t.Fatalf("publish = %v, %v", accepted, err)
	}
	if found, err := hub.EvictUser(ctx, "offline", uuid.New()); found || err != nil {
		t.Fatalf("evict = %v, %v", found, err)
	}
	if found, err := hub.CloseRoom(ctx, "offline"); found || err != nil {
		t.Fatalf("close = %v, %v", found, err)
	}
	if len(hub.rooms) != 0 {
		t.Fatal("server operation created an offline room")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := hub.PublishToRoom(canceled, "offline", ServerEvent{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled publish = %v", err)
	}
	if _, err := hub.CloseRoom(ctx, ""); !errors.Is(err, ErrInvalidRoom) {
		t.Fatalf("invalid room = %v", err)
	}
	if _, err := hub.EvictUser(ctx, "offline", uuid.Nil); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("invalid user = %v", err)
	}
}

func TestServerEventsPreserveOrderAndRecipientScope(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		room, err := hub.getOrCreate("conversation")
		if err != nil {
			t.Fatal(err)
		}
		first := joinRuntimeClient(t, room, uuid.New())
		second := joinRuntimeClient(t, room, uuid.New())
		drainRuntimeFrames(first)
		drainRuntimeFrames(second)

		payload := json.RawMessage(`{"eventSeq":1}`)
		publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.messageCreated", Payload: payload})
		// Changing the caller's buffer after enqueue must not change the frame.
		payload[len(payload)-2] = '9'
		publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.readUpdated", RecipientUserID: first.userID})
		publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.messageDeleted"})
		synctest.Wait()

		one := readRuntimeFrame(t, first)
		if one.Type != "chat.messageCreated" || string(one.Payload) != `{"eventSeq":1}` {
			t.Fatalf("first frame = %s %s", one.Type, one.Payload)
		}
		if got := readRuntimeFrame(t, first).Type; got != "chat.readUpdated" {
			t.Fatalf("private frame = %s", got)
		}
		if got := readRuntimeFrame(t, first).Type; got != "chat.messageDeleted" {
			t.Fatalf("last frame = %s", got)
		}
		if got := readRuntimeFrame(t, second).Type; got != "chat.messageCreated" {
			t.Fatalf("second user's first frame = %s", got)
		}
		if got := readRuntimeFrame(t, second).Type; got != "chat.messageDeleted" {
			t.Fatalf("private event leaked to another user: %s", got)
		}
		if len(second.send) != 0 {
			t.Fatal("unexpected additional recipient frame")
		}
	})
}

func TestServerEventsBoundQueueAndEncodedFrameSize(t *testing.T) {
	options := DefaultRoomOptions()
	options.ControlBufferSize = 1
	options.MaxEventBytes = 64
	hub, err := NewHubWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	// Do not start the loop until the queue checks are done. This makes queue
	// saturation deterministic without blocking a production module callback.
	room := newRoom("conversation", nil, nil, options)
	hub.rooms[room.id] = room
	t.Cleanup(func() {
		go room.Run()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	for _, event := range []ServerEvent{
		{Type: "invalid"}, {Type: ".changed"}, {Type: "chat."}, {Type: "system.presence"},
		{Type: "chat.changed", Payload: json.RawMessage(`{"bad":`)},
	} {
		if _, err := hub.PublishToRoom(ctx, room.id, event); !errors.Is(err, ErrInvalidServerEvent) {
			t.Fatalf("invalid event %#v: %v", event, err)
		}
	}
	// Raw type + payload fits 64 bytes, but the full encoded envelope does not.
	large := ServerEvent{Type: "chat.changed", Payload: json.RawMessage(`"` + strings.Repeat("x", 45) + `"`)}
	if _, err := hub.PublishToRoom(ctx, room.id, large); !errors.Is(err, ErrEventTooLarge) {
		t.Fatalf("encoded frame limit = %v", err)
	}
	publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.changed"})
	if _, err := hub.PublishToRoom(ctx, room.id, ServerEvent{Type: "chat.changed"}); !errors.Is(err, ErrRoomQueueFull) {
		t.Fatalf("full queue = %v", err)
	}
	if _, err := hub.EvictUser(ctx, room.id, uuid.New()); !errors.Is(err, ErrRoomQueueFull) {
		t.Fatalf("eviction must not pretend to be acknowledged: %v", err)
	}
	room.requestClose()
	if accepted, err := hub.PublishToRoom(ctx, room.id, ServerEvent{Type: "chat.changed"}); accepted || err != nil {
		t.Fatalf("closing room publish = %v, %v", accepted, err)
	}
}

func TestEvictionRemovesUserBeforeAckAndIgnoresStaleFrames(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		module := newScriptModule()
		if err := hub.Mount("tracking", func(string) (Module, error) { return module, nil }); err != nil {
			t.Fatal(err)
		}
		room, err := hub.getOrCreate("conversation")
		if err != nil {
			t.Fatal(err)
		}
		first := joinRuntimeClient(t, room, uuid.New())
		second := joinRuntimeClient(t, room, uuid.New())
		drainRuntimeFrames(first)
		drainRuntimeFrames(second)
		found, err := hub.EvictUser(context.Background(), room.id, first.userID)
		if !found || err != nil {
			t.Fatalf("evict = %v, %v", found, err)
		}
		if _, open := <-first.send; open {
			t.Fatal("evicted client's send channel remained open after ACK")
		}
		if module.leaveCount() != 1 {
			t.Fatal("eviction ACK preceded OnLeave")
		}
		// The old pump's unregister and already-buffered inbound data must not
		// remove another user, re-enter OnLeave, or invoke the module again.
		room.unregister <- first
		room.submit(inbound{client: first, env: Envelope{Type: "tracking.ping"}})
		if _, err := hub.EvictUser(context.Background(), room.id, first.userID); err != nil {
			t.Fatal(err)
		}
		publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.readUpdated", RecipientUserID: first.userID})
		publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.messageCreated"})
		synctest.Wait()
		if got := readRuntimeFrame(t, second).Type; got != MsgTypePresence {
			t.Fatalf("leave presence = %s", got)
		}
		if got := readRuntimeFrame(t, second).Type; got != "chat.messageCreated" {
			t.Fatalf("stale/private frame reached remaining user: %s", got)
		}
		if module.leaveCount() != 1 || len(second.send) != 0 {
			t.Fatal("stale unregister or message produced another callback/frame")
		}
	})
}

func TestEvictUserClosesRealSocket(t *testing.T) {
	hub := testRuntimeHub(t, time.Minute, 0)
	userID := uuid.New()
	conn := dialRoom(t, hub, "conversation", userID)
	_ = readOutgoing(t, conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if found, err := hub.EvictUser(ctx, "conversation", userID); !found || err != nil {
		t.Fatalf("evict = %v, %v", found, err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("evicted socket stayed open")
	}
}

func TestEvictionTimeoutDoesNotRetractQueuedCommand(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		options := DefaultRoomOptions()
		options.TickInterval = 0
		hub, err := NewHubWithOptions(options)
		if err != nil {
			t.Fatal(err)
		}
		room := newRoom("conversation", nil, nil, options)
		hub.rooms[room.id] = room
		// Populate before starting Run, then leave the command queued until the
		// caller times out. A later room turn must safely complete the eviction.
		client := newClient(room, nil, uuid.New())
		room.onJoin(client)
		drainRuntimeFrames(client)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if err := hub.Close(ctx); err != nil {
				t.Error(err)
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		found, err := hub.EvictUser(ctx, room.id, client.userID)
		go room.Run()
		if found || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unacknowledged eviction = %v, %v", found, err)
		}
		synctest.Wait()
		if _, open := <-client.send; open {
			t.Fatal("timed-out queued eviction was not applied later")
		}
		if _, err := hub.EvictUser(context.Background(), room.id, client.userID); err != nil {
			t.Fatalf("retry was not idempotent: %v", err)
		}
	})
}

func publishRuntimeEvent(t *testing.T, hub *Hub, event ServerEvent) {
	t.Helper()
	if accepted, err := hub.PublishToRoom(context.Background(), "conversation", event); !accepted || err != nil {
		t.Fatalf("publish = %v, %v", accepted, err)
	}
}

func readRuntimeFrame(t *testing.T, client *Client) Envelope {
	t.Helper()
	select {
	case frame, open := <-client.send:
		if !open {
			t.Fatal("client closed before expected frame")
		}
		var event Envelope
		if err := json.Unmarshal(frame, &event); err != nil {
			t.Fatal(err)
		}
		return event
	default:
		t.Fatal("expected frame not queued")
		return Envelope{}
	}
}

func drainRuntimeFrames(client *Client) {
	for len(client.send) > 0 {
		<-client.send
	}
}
