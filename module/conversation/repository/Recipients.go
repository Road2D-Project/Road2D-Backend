package repository

import (
	"context"

	"github.com/google/uuid"
)

func (r *ConversationRepository) Recipients(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Raw(`SELECT tm.user_id FROM trip_members tm JOIN conversations c
		ON c.trip_id = tm.trip_id WHERE c.id = ? AND tm.status = 'active' ORDER BY tm.user_id`, id).Scan(&ids).Error
	return ids, err
}

func (r *ConversationRepository) ConversationForTrip(ctx context.Context, tripID uuid.UUID) (uuid.UUID, error) {
	if r.db == nil {
		return uuid.Nil, ErrUnavailable
	}
	var scope struct{ ID uuid.UUID }
	err := r.db.WithContext(ctx).Raw("SELECT id FROM conversations WHERE trip_id = ?", tripID).Scan(&scope).Error
	return scope.ID, err
}
