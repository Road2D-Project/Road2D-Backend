package outbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrUnavailable = errors.New("outbox unavailable")

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// Claim leases rows in a short transaction. No row lock is held during delivery.
// An earlier pending row blocks later events of that conversation, even if it
// is backing off or leased by another processor.
func (r *Repository) Claim(ctx context.Context, limit int, lease time.Duration) ([]Event, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	if limit < 1 || limit > 200 || lease <= 0 {
		return nil, ErrUnavailable
	}
	token := uuid.New()
	var events []Event
	err := r.db.WithContext(ctx).Raw(`WITH candidates AS (
		SELECT e.id FROM outbox_events e
		WHERE e.dispatched_at IS NULL AND e.next_attempt_at <= clock_timestamp()
		AND (e.claimed_until IS NULL OR e.claimed_until <= clock_timestamp())
		AND NOT EXISTS (SELECT 1 FROM outbox_events prior
			WHERE prior.conversation_id = e.conversation_id AND prior.dispatched_at IS NULL
			AND prior.event_seq < e.event_seq)
		ORDER BY e.created_at, e.id LIMIT ? FOR UPDATE SKIP LOCKED)
		UPDATE outbox_events e SET claim_token = ?, claimed_until = clock_timestamp() + (? * INTERVAL '1 millisecond'),
		attempts = attempts + 1 FROM candidates WHERE e.id = candidates.id RETURNING e.*`,
		limit, token, lease.Milliseconds()).Scan(&events).Error
	return events, err
}

func (r *Repository) Complete(ctx context.Context, event Event) error {
	return r.db.WithContext(ctx).Model(&Event{}).Where("id = ? AND claim_token = ? AND dispatched_at IS NULL", event.ID, event.ClaimToken).
		Updates(map[string]any{"dispatched_at": gorm.Expr("clock_timestamp()"), "claim_token": nil, "claimed_until": nil}).Error
}

func (r *Repository) Retry(ctx context.Context, event Event) error {
	delay := time.Second << min(max(event.Attempts-1, 0), 8)
	return r.db.WithContext(ctx).Model(&Event{}).Where("id = ? AND claim_token = ? AND dispatched_at IS NULL", event.ID, event.ClaimToken).
		Updates(map[string]any{"next_attempt_at": gorm.Expr("clock_timestamp() + (? * INTERVAL '1 millisecond')", delay.Milliseconds()), "claim_token": nil, "claimed_until": nil}).Error
}
