package chatting

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func TestAdmissionCannotPassAnAccessChangeUsingAnOldSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hubs, err := NewChattingHubs(time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = hubs.Close(context.Background()) })
		reads := &accessStub{conversation: uuid.New(), user: uuid.New(), active: true}
		lifecycle := NewAccessLifecycle(hubs, reads)
		committed, release, changed, admitted := make(chan struct{}), make(chan struct{}), make(chan error, 1), make(chan error, 1)
		go func() {
			changed <- lifecycle.TripChange(context.Background(), uuid.New(), func(context.Context) error { reads.active = false; close(committed); <-release; return nil })
		}()
		<-committed
		go func() {
			admitted <- hubs.WithAccess(context.Background(), func() error {
				if reads.active {
					return nil
				}
				return realtime.ErrNotMember
			})
		}()
		synctest.Wait()
		select {
		case <-admitted:
			t.Fatal("admission raced the post-commit revocation")
		default:
		}
		close(release)
		if err := <-changed; err != nil {
			t.Fatal(err)
		}
		if err := <-admitted; !errors.Is(err, realtime.ErrNotMember) {
			t.Fatalf("admission reused stale authority: %v", err)
		}
	})
}

type accessStub struct {
	conversation, user uuid.UUID
	active             bool
}

func (r *accessStub) ConversationForTrip(context.Context, uuid.UUID) (uuid.UUID, error) {
	return r.conversation, nil
}
func (r *accessStub) Recipients(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	if r.active {
		return []uuid.UUID{r.user}, nil
	}
	return nil, nil
}
