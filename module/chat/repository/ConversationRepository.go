package repository

import (
	"context"
	"database/sql"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/request"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ConversationRepository struct{ db *gorm.DB }

func NewConversationRepository(db *gorm.DB) *ConversationRepository {
	return &ConversationRepository{db: db}
}

func (r *ConversationRepository) ListConversations(ctx context.Context, userID uuid.UUID, cursor *ConversationCursor, limit int) ([]ConversationRow, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	query := conversationSummarySQL + " SELECT * FROM summaries"
	args := []any{sql.Named("userId", userID), sql.Named("limit", limit+1)}
	if cursor != nil {
		query += " WHERE (last_activity_at, id) < (@activityAt, @cursorId)"
		args = append(args, sql.Named("activityAt", cursor.ActivityAt), sql.Named("cursorId", cursor.ID))
	}
	query += " ORDER BY last_activity_at DESC, id DESC LIMIT @limit"
	var rows []ConversationRow
	err := r.db.WithContext(ctx).Raw(query, args...).Scan(&rows).Error
	return rows, err
}

func (r *ConversationRepository) GetConversation(ctx context.Context, userID, conversationID uuid.UUID) (*ConversationRow, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	var row ConversationRow
	result := r.db.WithContext(ctx).Raw(conversationSummarySQL+" SELECT * FROM summaries WHERE id = @conversationId",
		sql.Named("userId", userID), sql.Named("conversationId", conversationID)).Scan(&row)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, ErrConversationNotFound
	}
	return &row, nil
}

func (r *ConversationRepository) ListMessages(ctx context.Context, userID, conversationID uuid.UUID, query request.ListMessagesRequest) ([]model.Message, error) {
	if r.db == nil {
		return nil, ErrUnavailable
	}
	var messages []model.Message
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var allowed bool
		if err := tx.Raw(`SELECT EXISTS (SELECT 1 FROM conversations c JOIN trip_members tm
			ON tm.trip_id = c.trip_id WHERE c.id = ? AND tm.user_id = ? AND tm.status = 'active')`,
			conversationID, userID).Scan(&allowed).Error; err != nil {
			return err
		}
		if !allowed {
			return ErrConversationNotFound
		}
		rows := tx.Where("conversation_id = ?", conversationID).Limit(query.Limit + 1)
		if query.AfterSeq != nil {
			rows = rows.Where("seq > ?", *query.AfterSeq).Order("seq ASC")
		} else {
			rows = rows.Order("seq DESC")
			if query.BeforeSeq != nil {
				rows = rows.Where("seq < ?", *query.BeforeSeq)
			}
		}
		return rows.Find(&messages).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return messages, err
}
