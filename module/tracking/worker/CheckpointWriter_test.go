package worker

import (
	"context"
	"errors"
	"testing"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
)

type checkpointStoreFunc func(context.Context, model.TripCheckpoint) error

func (f checkpointStoreFunc) Save(ctx context.Context, checkpoint model.TripCheckpoint) error {
	return f(ctx, checkpoint)
}

func TestCheckpointWriterCoalescesAndRetriesNewestSnapshot(t *testing.T) {
	tripID := uuid.New()
	memberID := uuid.New()
	writeError := errors.New("database temporarily unavailable")
	fail := true
	var saved []model.TripCheckpoint
	writer := NewCheckpointWriter(checkpointStoreFunc(func(_ context.Context, checkpoint model.TripCheckpoint) error {
		if fail {
			return writeError
		}
		saved = append(saved, checkpoint)
		return nil
	}))
	writer.Offer(model.TripCheckpoint{TripID: tripID, Sequence: 7})
	if err := writer.Flush(context.Background()); !errors.Is(err, writeError) {
		t.Fatalf("got %v, want write error", err)
	}
	writer.Offer(model.TripCheckpoint{
		TripID: tripID, Sequence: 7, Riders: map[uuid.UUID]model.RiderCheckpoint{memberID: {Lat: 12}},
	})
	fail = false
	if err := writer.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Sequence != 9 || saved[0].Riders[memberID].Lat != 12 {
		t.Fatalf("saved %#v", saved)
	}
	// A recreated room may have loaded a checkpoint older than an outstanding write.
	writer.Offer(model.TripCheckpoint{TripID: tripID, Sequence: 7})
	if err := writer.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if saved[1].Sequence != 10 {
		t.Fatalf("room recreation regressed sequence to %d", saved[1].Sequence)
	}
}

func TestCheckpointOfferedDuringSaveRemainsPending(t *testing.T) {
	tripID := uuid.New()
	started := make(chan struct{})
	release := make(chan struct{})
	var saved []uint64
	writer := NewCheckpointWriter(checkpointStoreFunc(func(_ context.Context, checkpoint model.TripCheckpoint) error {
		saved = append(saved, checkpoint.Sequence)
		if len(saved) == 1 {
			close(started)
			<-release
		}
		return nil
	}))
	writer.Offer(model.TripCheckpoint{TripID: tripID})
	done := make(chan error, 1)
	go func() { done <- writer.Flush(context.Background()) }()
	<-started
	writer.Offer(model.TripCheckpoint{TripID: tripID})
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := writer.Flush(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(saved) != 2 || saved[0] != 1 || saved[1] != 2 {
		t.Fatalf("saved sequences %v", saved)
	}
}
