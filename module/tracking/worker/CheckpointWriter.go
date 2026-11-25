package worker

import (
	"context"
	"log"
	"sync"
	"time"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
)

type CheckpointStore interface {
	Save(context.Context, model.TripCheckpoint) error
}

// CheckpointWriter keeps at most one pending immutable snapshot per trip.
// The shared sequence also orders writes across idle room recreation.
type CheckpointWriter struct {
	store     CheckpointStore
	mu        sync.Mutex
	pending   map[uuid.UUID]model.TripCheckpoint
	sequences map[uuid.UUID]uint64
	wake      chan struct{}
}

func NewCheckpointWriter(store CheckpointStore) *CheckpointWriter {
	return &CheckpointWriter{
		store: store, pending: make(map[uuid.UUID]model.TripCheckpoint),
		sequences: make(map[uuid.UUID]uint64), wake: make(chan struct{}, 1),
	}
}

// Offer takes ownership of the snapshot. The caller must not mutate its maps or slices.
func (w *CheckpointWriter) Offer(checkpoint model.TripCheckpoint) {
	w.mu.Lock()
	sequence := max(w.sequences[checkpoint.TripID], checkpoint.Sequence) + 1
	w.sequences[checkpoint.TripID] = sequence
	checkpoint.Sequence = sequence
	w.pending[checkpoint.TripID] = checkpoint
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *CheckpointWriter) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
		if err := w.Flush(ctx); err != nil && ctx.Err() == nil {
			log.Printf("tracking checkpoint: %v", err)
		}
	}
}

// Flush removes a snapshot only after saving succeeds. A newer snapshot offered
// during I/O remains pending. Run must stop before the final shutdown Flush.
func (w *CheckpointWriter) Flush(ctx context.Context) error {
	w.mu.Lock()
	batch := make([]model.TripCheckpoint, 0, len(w.pending))
	for _, checkpoint := range w.pending {
		batch = append(batch, checkpoint)
	}
	w.mu.Unlock()
	for _, checkpoint := range batch {
		writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := w.store.Save(writeCtx, checkpoint)
		cancel()
		if err != nil {
			return err
		}
		w.mu.Lock()
		if w.pending[checkpoint.TripID].Sequence == checkpoint.Sequence {
			delete(w.pending, checkpoint.TripID)
		}
		w.mu.Unlock()
	}
	return nil
}
