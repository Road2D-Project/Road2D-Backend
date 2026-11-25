package repository

import (
	"context"
	"errors"

	"Road-To-Destination-BE/module/chat/model"
	"gorm.io/gorm"
)

func (r *MutationRepository) GetUserState(ctx context.Context) (*model.ConversationUserState, error) {
	var state model.ConversationUserState
	err := r.db.WithContext(ctx).Where("conversation_id = ? AND user_id = ?", r.Conversation.ID, r.Member.UserID).First(&state).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return &state, err
	}
	state = model.ConversationUserState{ConversationID: r.Conversation.ID, UserID: r.Member.UserID}
	if err := r.db.WithContext(ctx).Raw(`SELECT COALESCE(MAX(seq),0) FROM messages WHERE conversation_id = ? AND created_at < ?`, r.Conversation.ID, r.Member.JoinedAt).Scan(&state.LastReadSeq).Error; err != nil {
		return nil, err
	}
	return &state, r.db.WithContext(ctx).Create(&state).Error
}

func (r *MutationRepository) SaveUserState(ctx context.Context, state *model.ConversationUserState) error {
	return r.db.WithContext(ctx).Model(&model.ConversationUserState{}).Where("id = ?", state.ID).
		Updates(map[string]any{"last_read_seq": gorm.Expr("GREATEST(last_read_seq, ?)", state.LastReadSeq), "muted_until": state.MutedUntil}).Error
}

func (r *MutationRepository) Summary(ctx context.Context) (*ConversationRow, error) {
	return NewConversationRepository(r.db).GetConversation(ctx, r.Member.UserID, r.Conversation.ID)
}
