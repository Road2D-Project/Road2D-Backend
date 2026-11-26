package realtime

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
)

func TestRoomOptionsKeepTrackingDefaultsAndRejectInvalidValues(t *testing.T) {
	defaults := DefaultRoomOptions()
	if defaults.TickInterval != 500*time.Millisecond || defaults.IdleTimeout != 5*time.Minute {
		t.Fatalf("tracking defaults changed: %#v", defaults)
	}
	for _, change := range []func(*RoomOptions){
		func(o *RoomOptions) { o.IdleTimeout = 0 },
		func(o *RoomOptions) { o.TickInterval = -time.Second },
		func(o *RoomOptions) { o.ControlBufferSize = 0 },
		func(o *RoomOptions) { o.MaxEventBytes = 0 },
	} {
		options := defaults
		change(&options)
		if _, err := NewHubWithOptions(options); !errors.Is(err, ErrInvalidRoomOptions) {
			t.Fatalf("invalid options %#v = %v", options, err)
		}
	}
}

func TestIdleRoomExpiresWithoutTicksAndPublishDoesNotExtendLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		module := &runtimeClockModule{closed: make(chan struct{})}
		if err := hub.Mount("chat", func(string) (Module, error) { return module, nil }); err != nil {
			t.Fatal(err)
		}
		room, err := hub.getOrCreate("conversation")
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		for range 2 {
			time.Sleep(20 * time.Second)
			publishRuntimeEvent(t, hub, ServerEvent{Type: "chat.changed"})
			synctest.Wait()
		}
		time.Sleep(20 * time.Second)
		synctest.Wait()
		select {
		case <-room.done:
		default:
			t.Fatal("publication kept an empty room alive")
		}
		select {
		case <-module.closed:
		default:
			t.Fatal("idle cleanup skipped module Close")
		}
		if module.ticks != 0 {
			t.Fatalf("disabled ticks ran %d times", module.ticks)
		}
		if accepted, err := hub.PublishToRoom(context.Background(), room.id, ServerEvent{Type: "chat.changed"}); accepted || err != nil {
			t.Fatalf("publish resurrected idle room: %v, %v", accepted, err)
		}
		// Use fresh factories on the new room; each factory owns its own state.
		hub.factories["chat"] = func(string) (Module, error) { return nopModule{}, nil }
		fresh, err := hub.getOrCreate(room.id)
		if err != nil || fresh == room {
			t.Fatalf("fresh room = %p, %v", fresh, err)
		}
		hub.remove(room.id, room)
		stored, err := hub.existingRoom(context.Background(), room.id)
		if err != nil || stored != fresh {
			t.Fatal("old cleanup removed the replacement room")
		}
		if found, err := hub.CloseRoom(context.Background(), room.id); !found || err != nil {
			t.Fatalf("close replacement room = %v, %v", found, err)
		}
		if found, err := hub.CloseRoom(context.Background(), room.id); found || err != nil {
			t.Fatalf("repeated close = %v, %v", found, err)
		}
		if fresh.join(newClient(fresh, nil, uuid.New())) || fresh.submit(inbound{}) {
			t.Fatal("completed room accepted a join or inbound frame")
		}
	})
}

func TestJoinCancelsIdleAndLastLeaveOrEvictionRestartsIt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		room, err := hub.getOrCreate("conversation")
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(30 * time.Second)
		client := joinRuntimeClient(t, room, uuid.New())
		time.Sleep(2 * time.Minute)
		assertRuntimeOpen(t, room)
		room.unregister <- client
		synctest.Wait()
		time.Sleep(30 * time.Second)
		client = joinRuntimeClient(t, room, client.userID)
		time.Sleep(2 * time.Minute)
		assertRuntimeOpen(t, room)
		if _, err := hub.EvictUser(context.Background(), room.id, client.userID); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(59 * time.Second)
		assertRuntimeOpen(t, room)
		time.Sleep(time.Second)
		synctest.Wait()
		select {
		case <-room.done:
		default:
			t.Fatal("last eviction did not start idle cleanup")
		}
	})
}

func TestConfiguredTickRunsWhileRoomIsEmpty(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 500*time.Millisecond)
		module := &runtimeClockModule{closed: make(chan struct{})}
		if err := hub.Mount("tracking", func(string) (Module, error) { return module, nil }); err != nil {
			t.Fatal(err)
		}
		if _, err := hub.getOrCreate("trip"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		time.Sleep(time.Second)
		synctest.Wait()
		if module.ticks != 2 {
			t.Fatalf("ticks = %d, want 2", module.ticks)
		}
	})
}

func TestTicklessRoomDoesNotAccumulateLatestBatches(t *testing.T) {
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
		client := joinRuntimeClient(t, room, uuid.New())
		room.submit(inbound{client: client, env: Envelope{Type: "tracking.location", Payload: []byte(`{"lat":1}`)}})
		synctest.Wait()
		if len(room.latest) != 0 {
			t.Fatal("tickless room retained a batch that cannot be flushed")
		}
	})
}

func TestCloseRoomWaitsForCleanupAndRejectsNewWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		gate := make(chan struct{})
		defer close(gate)
		module := &blockedCloseModule{started: make(chan struct{}), release: gate}
		if err := hub.Mount("chat", func(string) (Module, error) { return module, nil }); err != nil {
			t.Fatal(err)
		}
		room, err := hub.getOrCreate("conversation")
		if err != nil {
			t.Fatal(err)
		}
		joinRuntimeClient(t, room, uuid.New())
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if found, err := hub.CloseRoom(ctx, room.id); !found || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("close must wait for module cleanup: %v, %v", found, err)
		}
		if accepted, err := hub.PublishToRoom(context.Background(), room.id, ServerEvent{Type: "chat.changed"}); accepted || err != nil {
			t.Fatalf("closing room accepted event: %v, %v", accepted, err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err := hub.EvictUser(ctx, room.id, uuid.New()); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("closing room acknowledged eviction before cleanup: %v", err)
		}
	})
}

func TestHubShutdownSignalsAllRoomsBeforeWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		gate := make(chan struct{})
		defer close(gate)
		blocked := &blockedCloseModule{started: make(chan struct{}), release: gate}
		quick := &runtimeClockModule{closed: make(chan struct{})}
		if err := hub.Mount("chat", func(id string) (Module, error) {
			if id == "blocked" {
				return blocked, nil
			}
			return quick, nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"blocked", "quick"} {
			if _, err := hub.getOrCreate(id); err != nil {
				t.Fatal(err)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := hub.Close(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("shutdown = %v", err)
		}
		synctest.Wait()
		select {
		case <-quick.closed:
		default:
			t.Fatal("slow room prevented the other room from closing")
		}
		if _, err := hub.PublishToRoom(context.Background(), "quick", ServerEvent{Type: "chat.changed"}); !errors.Is(err, ErrRoomClosed) {
			t.Fatalf("closed hub accepted publication: %v", err)
		}
	})
}

type runtimeClockModule struct {
	nopModule
	ticks  int
	closed chan struct{}
}

func (m *runtimeClockModule) OnTick(Session, time.Time) { m.ticks++ }
func (m *runtimeClockModule) Close()                    { close(m.closed) }

type blockedCloseModule struct {
	nopModule
	started chan struct{}
	release <-chan struct{}
}

func (m *blockedCloseModule) Close() {
	close(m.started)
	<-m.release
}

func testRuntimeHub(t *testing.T, idle, tick time.Duration) *Hub {
	t.Helper()
	options := DefaultRoomOptions()
	options.IdleTimeout, options.TickInterval = idle, tick
	hub, err := NewHubWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := hub.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	return hub
}

// These clients intentionally have no network pumps. Tests drive register,
// unregister and queues inside a synctest bubble to check timers without sleeps
// in real time. Real socket closure is covered separately.
func joinRuntimeClient(t *testing.T, room *Room, userID uuid.UUID) *Client {
	t.Helper()
	client := newClient(room, nil, userID)
	if !room.join(client) {
		t.Fatal("room rejected client")
	}
	synctest.Wait()
	return client
}

func assertRuntimeOpen(t *testing.T, room *Room) {
	t.Helper()
	select {
	case <-room.done:
		t.Fatal("room with a live client expired")
	default:
	}
}
