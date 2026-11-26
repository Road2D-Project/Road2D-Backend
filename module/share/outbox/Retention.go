package outbox

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Prune updates the durable replay floor in the same transaction as deletion.
// Lock parents before event rows, matching trip/conversation deletion order.
// Pending events are never deleted, regardless of their age.
func (r *Repository) Prune(ctx context.Context, limit int) (int, error) {
	if r.db == nil {
		return 0, ErrUnavailable
	}
	if limit < 1 || limit > 1000 {
		return 0, ErrUnavailable
	}
	deleted := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ids []uuid.UUID
		if err := tx.Raw(`SELECT c.id FROM conversations c WHERE EXISTS
			(SELECT 1 FROM outbox_events e WHERE e.conversation_id = c.id AND e.dispatched_at IS NOT NULL
			AND e.created_at < CURRENT_TIMESTAMP - INTERVAL '7 days')
			ORDER BY c.id LIMIT 16 FOR UPDATE SKIP LOCKED`).Scan(&ids).Error; err != nil {
			return err
		}
		remaining := limit
		for _, id := range ids {
			var seqs []int64
			if err := tx.Raw(`WITH doomed AS (SELECT id FROM outbox_events WHERE conversation_id = ?
				AND dispatched_at IS NOT NULL AND created_at < CURRENT_TIMESTAMP - INTERVAL '7 days'
				ORDER BY event_seq LIMIT ? FOR UPDATE SKIP LOCKED)
				DELETE FROM outbox_events e USING doomed WHERE e.id = doomed.id RETURNING e.event_seq`, id, remaining).Scan(&seqs).Error; err != nil {
				return err
			}
			var floor int64
			for _, seq := range seqs {
				floor = max(floor, seq)
			}
			if floor > 0 {
				if err := tx.Exec("UPDATE conversations SET replay_floor = GREATEST(replay_floor, ?) WHERE id = ?", floor, id).Error; err != nil {
					return err
				}
			}
			remaining -= len(seqs)
			deleted += len(seqs)
			if remaining == 0 {
				break
			}
		}
		result := tx.Exec(`DELETE FROM outbox_events WHERE id IN (SELECT id FROM outbox_events
			WHERE conversation_id IS NULL AND dispatched_at IS NOT NULL
			AND created_at < CURRENT_TIMESTAMP - INTERVAL '7 days' ORDER BY created_at LIMIT ? FOR UPDATE SKIP LOCKED)`, remaining)
		deleted += int(result.RowsAffected)
		return result.Error
	})
	if err != nil {
		return 0, err
	}
	return deleted, nil
}

func (r *Repository) RunRetention(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		for ctx.Err() == nil {
			n, err := r.Prune(ctx, 200)
			if err != nil {
				if ctx.Err() == nil {
					log.Printf("outbox: retention: %v", err)
				}
				break
			}
			if n == 0 {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
