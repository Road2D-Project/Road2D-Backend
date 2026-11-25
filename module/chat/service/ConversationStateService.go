package service

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/model/response"
	"github.com/google/uuid"
)

type ConversationStateService struct{ transaction ChatTransaction }

func NewConversationStateService(transaction ChatTransaction) *ConversationStateService {
	return &ConversationStateService{transaction: transaction}
}

func (s *ConversationStateService) MarkRead(ctx context.Context, actor, id uuid.UUID, req request.MarkReadRequest) (*response.ConversationResponse, error) {
	if req.LastReadSeq == nil || *req.LastReadSeq < 0 {
		return nil, ErrInvalidInput
	}
	var result response.ConversationResponse
	err := s.transaction(ctx, actor, id, func(tx ChatWrite) error {
		if *req.LastReadSeq >= tx.Conversation.NextMessageSeq {
			return ErrInvalidInput
		}
		state, err := tx.State.GetUserState(ctx)
		if err != nil {
			return err
		}
		if *req.LastReadSeq > state.LastReadSeq {
			state.LastReadSeq = *req.LastReadSeq
			if err := tx.State.SaveUserState(ctx, state); err != nil {
				return err
			}
			if err := tx.Events.AppendEvent(ctx, model.EventReadUpdated, id, &actor, model.ConversationEventPayload{UserID: &actor}); err != nil {
				return err
			}
		}
		row, err := tx.State.Summary(ctx)
		if err == nil {
			result = conversationResponse(*row)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *ConversationStateService) UpdatePreferences(ctx context.Context, actor, id uuid.UUID, req request.UpdatePreferencesRequest) (*response.ConversationResponse, error) {
	if len(req.MutedUntil) == 0 {
		return nil, ErrInvalidInput
	}
	var until *time.Time
	if !bytes.Equal(bytes.TrimSpace(req.MutedUntil), []byte("null")) {
		var parsed time.Time
		if json.Unmarshal(req.MutedUntil, &parsed) != nil || parsed.IsZero() {
			return nil, ErrInvalidInput
		}
		parsed = parsed.UTC().Truncate(time.Microsecond)
		until = &parsed
	}
	var result response.ConversationResponse
	err := s.transaction(ctx, actor, id, func(tx ChatWrite) error {
		if until != nil && !until.After(tx.Now) {
			return ErrInvalidInput
		}
		state, err := tx.State.GetUserState(ctx)
		if err != nil {
			return err
		}
		same := state.MutedUntil == nil && until == nil || state.MutedUntil != nil && until != nil && state.MutedUntil.Equal(*until)
		if !same {
			state.MutedUntil = until
			if err := tx.State.SaveUserState(ctx, state); err != nil {
				return err
			}
			if err := tx.Events.AppendEvent(ctx, model.EventPreferencesUpdated, id, &actor, model.ConversationEventPayload{UserID: &actor}); err != nil {
				return err
			}
		}
		row, err := tx.State.Summary(ctx)
		if err == nil {
			result = conversationResponse(*row)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
