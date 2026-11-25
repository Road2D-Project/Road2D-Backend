package repository

import (
	"context"
	"database/sql"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/model/response"
	"Road-To-Destination-BE/module/share/outbox"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *ConversationRepository) ListEvents(ctx context.Context, actor, id uuid.UUID, query request.ListEventsRequest) (*response.EventListResponse, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	out := &response.EventListResponse{Events: []response.ConversationEventResponse{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var scope struct{ NextEventSeq int64 }
		result := tx.Table("conversations c").Select("c.next_event_seq").Joins("JOIN trip_members tm ON tm.trip_id = c.trip_id").
			Where("c.id = ? AND tm.user_id = ? AND tm.status = 'active'", id, actor).Scan(&scope)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrConversationNotFound
		}
		out.LatestEventSeq = scope.NextEventSeq - 1
		if query.AfterEventSeq > out.LatestEventSeq {
			return ErrInvalidCursor
		}
		var first sql.NullInt64
		var lastExpired int64
		if err := tx.Raw(`SELECT
			MIN(event_seq) FILTER (WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '7 days'),
			COALESCE(MAX(event_seq) FILTER (WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '7 days'), 0)
			FROM outbox_events WHERE conversation_id = ?`, id).Row().Scan(&first, &lastExpired); err != nil {
			return err
		}
		out.MinAfterEventSeq = out.LatestEventSeq
		if first.Valid {
			// Clock adjustments must not silently skip an expired event whose
			// sequence is newer than an earlier retained event.
			out.MinAfterEventSeq = max(first.Int64-1, lastExpired)
		}
		if query.AfterEventSeq < out.MinAfterEventSeq {
			return &ReplayExpiredError{MinAfterEventSeq: out.MinAfterEventSeq, LatestEventSeq: out.LatestEventSeq}
		}
		var rows []outbox.Event
		err := tx.Where("conversation_id = ? AND event_seq > ? AND event_seq <= ?", id, query.AfterEventSeq, out.LatestEventSeq).
			Where("created_at >= CURRENT_TIMESTAMP - INTERVAL '7 days'").
			Where("recipient_user_id IS NULL OR recipient_user_id = ?", actor).
			Where("type IN ?", []string{model.EventMessageCreated, model.EventMessageUpdated, model.EventMessageDeleted, model.EventReadUpdated, model.EventPreferencesUpdated}).
			Order("event_seq ASC").Limit(query.Limit + 1).Find(&rows).Error
		if err != nil {
			return err
		}
		out.HasMore = len(rows) > query.Limit
		if out.HasMore {
			rows = rows[:query.Limit]
		}
		out.NextEventSeq = out.LatestEventSeq
		for _, row := range rows {
			out.Events = append(out.Events, response.ConversationEventResponse{EventID: row.ID, Type: row.Type, ConversationID: id,
				EventSeq: *row.EventSeq, Payload: row.Payload, CreatedTime: row.CreatedAt})
		}
		if out.HasMore {
			out.NextEventSeq = *rows[len(rows)-1].EventSeq
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	return out, nil
}
