package realtime

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
)

func TestSessionRevocationDoesNotEvictAnotherLoginOrNewerVersion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		user, other := uuid.New(), uuid.New()
		clients := make([]*Client, 3)
		for i, actor := range []uuid.UUID{user, user, other} {
			room, err := hub.getOrCreate(uuid.NewString())
			if err != nil {
				t.Fatal(err)
			}
			client := newClient(room, nil, actor)
			client.options = ConnectionOptions{SessionID: "old-login", SessionVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}
			if i == 1 {
				client.options.SessionID = "new-login"
				client.options.SessionVersion = 2
			}
			if !room.join(client) {
				t.Fatal("join failed")
			}
			synctest.Wait()
			drainRuntimeFrames(client)
			clients[i] = client
		}
		if err := hub.EvictSession(context.Background(), user, SessionFilter{SessionID: "old-login"}); err != nil {
			t.Fatal(err)
		}
		if _, open := <-clients[0].send; open {
			t.Fatal("matching login remained open")
		}
		for _, i := range []int{1, 2} {
			select {
			case <-clients[i].send:
				t.Fatal("revocation evicted another login/user")
			default:
			}
		}
		if err := hub.EvictSession(context.Background(), user, SessionFilter{BeforeVersion: 2}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-clients[1].send:
			t.Fatal("old revocation evicted a newer version")
		default:
		}
	})
}

func TestCanceledRevocationStillClosesRoomsThroughFallback(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := testRuntimeHub(t, time.Minute, 0)
		room, err := hub.getOrCreate("fallback")
		if err != nil {
			t.Fatal(err)
		}
		user := uuid.New()
		client := joinRuntimeClient(t, room, user)
		drainRuntimeFrames(client)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := hub.EvictSession(ctx, user, SessionFilter{BeforeVersion: 2}); err != nil {
			t.Fatal(err)
		}
		select {
		case <-room.done:
		default:
			t.Fatal("canceled request skipped fallback cleanup")
		}
		if _, open := <-client.send; open {
			t.Fatal("fallback left a socket registered")
		}
	})
}
