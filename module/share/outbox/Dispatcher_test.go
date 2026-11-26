package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDispatcherRetriesFailedHandoffsWithoutCompletingThem(t *testing.T) {
	store := &dispatchStub{events: []Event{{Type: "test"}}}
	handler := dispatchHandler(func(context.Context, Event) error { return errors.New("queue full") })
	if _, err := NewDispatcher(store, handler).ProcessBatch(t.Context()); err != nil {
		t.Fatal(err)
	}
	if store.completed != 0 || store.retried != 1 {
		t.Fatalf("completed=%d retries=%d", store.completed, store.retried)
	}
}

func TestDispatcherShutdownFinishesAcceptedHandoffBeforeReturning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	store := &dispatchStub{events: []Event{{AggregateID: uuid.New()}}}
	d := NewDispatcher(store, dispatchHandler(func(work context.Context, _ Event) error {
		close(entered)
		<-release
		if work.Err() != nil {
			return work.Err()
		}
		return nil
	}))
	go func() { defer close(done); d.Run(ctx) }()
	<-entered
	cancel()
	select {
	case <-done:
		t.Fatal("shutdown discarded an accepted event")
	default:
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not drain")
	}
	if store.completed != 1 || store.retried != 0 {
		t.Fatalf("handoff was not acknowledged: %#v", store)
	}
}

type dispatchHandler func(context.Context, Event) error

func (f dispatchHandler) Handle(ctx context.Context, e Event) error { return f(ctx, e) }

type dispatchStub struct {
	events             []Event
	completed, retried int
}

func (s *dispatchStub) Claim(context.Context, int, time.Duration) ([]Event, error) {
	events := s.events
	s.events = nil
	return events, nil
}
func (s *dispatchStub) Complete(context.Context, Event) error { s.completed++; return nil }
func (s *dispatchStub) Retry(context.Context, Event) error    { s.retried++; return nil }
