package outbox

import (
	"context"
	"log"
	"time"
)

type DispatchStore interface {
	Claim(context.Context, int, time.Duration) ([]Event, error)
	Complete(context.Context, Event) error
	Retry(context.Context, Event) error
}

type EventHandler interface {
	Handle(context.Context, Event) error
}

type Dispatcher struct {
	store   DispatchStore
	handler EventHandler
}

func NewDispatcher(store DispatchStore, handler EventHandler) *Dispatcher {
	return &Dispatcher{store: store, handler: handler}
}

// Run stops claiming on cancellation, while one accepted event may finish its
// bounded handoff. Keep the hubs open until Run returns during shutdown.
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for ctx.Err() == nil {
		n, err := d.ProcessBatch(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("outbox: dispatch: %v", err)
		}
		if err == nil && n > 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) ProcessBatch(ctx context.Context) (int, error) {
	// One in-flight lease keeps the worst-case 5s handoff + 2s ACK below
	// the 30s lease. Leasing a large serial batch could expire its tail before
	// it starts, allowing a second worker to overtake it.
	events, err := d.store.Claim(ctx, 1, 30*time.Second)
	if err != nil {
		return 0, err
	}
	for _, event := range events {
		if ctx.Err() != nil {
			break
		}
		work, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		err := d.handler.Handle(work, event)
		cancel()
		ack, cancelAck := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		if err == nil {
			err = d.store.Complete(ack, event)
		} else {
			log.Printf("outbox: event %s attempt %d: %v", event.ID, event.Attempts, err)
			err = d.store.Retry(ack, event)
		}
		cancelAck()
		if err != nil {
			return len(events), err
		}
	}
	return len(events), nil
}
