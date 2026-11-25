package realtime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type closingModule struct {
	nopModule
	closed chan struct{}
}

func (m *closingModule) Close() { close(m.closed) }

func TestHubCloseFinishesModulesBeforeReturning(t *testing.T) {
	hub := NewHub()
	module := &closingModule{closed: make(chan struct{})}
	if err := hub.Mount("tracking", func(string) (Module, error) { return module, nil }); err != nil {
		t.Fatal(err)
	}
	room, err := hub.getOrCreate("trip")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := hub.Close(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-module.closed:
	default:
		t.Fatal("module not closed before hub shutdown completed")
	}
	select {
	case <-room.done:
	default:
		t.Fatal("room pumps were not released")
	}
	if _, err := hub.getOrCreate("another-trip"); !errors.Is(err, ErrRoomClosed) {
		t.Fatalf("closed hub accepted a new room: %v", err)
	}
}
