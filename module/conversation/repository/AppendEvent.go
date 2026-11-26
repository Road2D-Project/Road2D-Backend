package repository

import (
	"context"
	"encoding/json"

	"Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/share/outbox"
	"github.com/google/uuid"
)

func (r *MutationRepository) AppendEvent(ctx context.Context, kind string, target uuid.UUID, recipient *uuid.UUID, payload model.ConversationEventPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	seq := r.Conversation.NextEventSeq
	event := outbox.Event{Type: kind, AggregateID: target, ConversationID: &r.Conversation.ID,
		RecipientUserID: recipient, EventSeq: &seq, Payload: encoded}
	event.CreatedAt, event.UpdatedAt = r.Now, r.Now
	if err := r.db.WithContext(ctx).Create(&event).Error; err != nil {
		return err
	}
	r.Conversation.NextEventSeq++
	// Persist both watermarks before querying an acknowledgement summary in this tx.
	return r.db.WithContext(ctx).Model(&model.Conversation{}).Where("id = ?", r.Conversation.ID).
		Updates(map[string]any{"next_message_seq": r.Conversation.NextMessageSeq, "next_event_seq": r.Conversation.NextEventSeq}).Error
}
