package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/conversation/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (r *MutationRepository) FindClientMessage(ctx context.Context, clientID uuid.UUID) (*model.Message, error) {
	var row model.Message
	err := r.db.WithContext(ctx).Where("conversation_id = ? AND sender_id = ? AND client_message_id = ?", r.Conversation.ID, r.Member.UserID, clientID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &row, err
}

func (r *MutationRepository) FindMessage(ctx context.Context, id uuid.UUID) (*model.Message, error) {
	var row model.Message
	err := r.db.WithContext(ctx).Where("conversation_id = ? AND id = ?", r.Conversation.ID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrMessageNotFound
	}
	return &row, err
}

func (r *MutationRepository) InsertMessage(ctx context.Context, row *model.Message) error {
	return r.db.WithContext(ctx).Create(row).Error
}

func (r *MutationRepository) SaveMessage(ctx context.Context, row *model.Message) error {
	return r.db.WithContext(ctx).Model(&model.Message{}).Where("id = ? AND conversation_id = ?", row.ID, r.Conversation.ID).
		Updates(map[string]any{"text": row.Text, "revision": row.Revision, "edited_at": row.EditedAt, "deleted_at": row.DeletedAt}).Error
}
