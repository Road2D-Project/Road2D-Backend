package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/model/response"
	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
)

type MessageService struct{ transaction ChatTransaction }

func NewMessageService(transaction ChatTransaction) *MessageService {
	return &MessageService{transaction: transaction}
}

func validMessageText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || !utf8.ValidString(text) || utf8.RuneCountInString(text) > 4000 {
		return "", ErrInvalidInput
	}
	return text, nil
}

func (s *MessageService) CreateMessage(ctx context.Context, actor, conversationID uuid.UUID, req request.CreateMessageRequest) (*response.MessageResponse, bool, error) {
	text, err := validMessageText(req.Text)
	if err != nil || req.ClientMessageID == uuid.Nil || (req.ReplyToMessageID != nil && *req.ReplyToMessageID == uuid.Nil) {
		return nil, false, ErrInvalidInput
	}
	req.Text = text
	body, err := json.Marshal(struct {
		Text    string     `json:"text"`
		ReplyTo *uuid.UUID `json:"replyToMessageId"`
	}{text, req.ReplyToMessageID})
	if err != nil {
		return nil, false, err
	}
	hash := sha256.Sum256(body)
	requestHash := hex.EncodeToString(hash[:])
	var result response.MessageResponse
	var created bool
	err = s.transaction(ctx, actor, conversationID, func(tx ChatWrite) error {
		created = false
		previous, err := tx.Messages.FindClientMessage(ctx, req.ClientMessageID)
		if err != nil {
			return err
		}
		if previous != nil {
			if previous.RequestHash != requestHash {
				return ErrIdempotencyConflict
			}
			result = messageResponse(*previous)
			return nil
		}
		if req.ReplyToMessageID != nil {
			if _, err := tx.Messages.FindMessage(ctx, *req.ReplyToMessageID); err != nil {
				return err
			}
		}
		msg := &model.Message{ConversationID: conversationID, SenderID: &actor, Seq: tx.Conversation.NextMessageSeq,
			Kind: enum.MessageKindText, Text: &text, ReplyToMessageID: req.ReplyToMessageID, ClientMessageID: &req.ClientMessageID, RequestHash: requestHash, Revision: 1}
		msg.CreatedAt, msg.UpdatedAt = tx.Now, tx.Now
		if err := tx.Messages.InsertMessage(ctx, msg); err != nil {
			return err
		}
		tx.Conversation.NextMessageSeq++
		if err := tx.Events.AppendEvent(ctx, model.EventMessageCreated, msg.ID, nil, messageEvent(msg)); err != nil {
			return err
		}
		result, created = messageResponse(*msg), true
		return nil
	})
	if err != nil {
		return nil, false, err
	}
	return &result, created, nil
}

func (s *MessageService) UpdateMessage(ctx context.Context, actor, conversationID, id uuid.UUID, req request.UpdateMessageRequest) (*response.MessageResponse, error) {
	text, err := validMessageText(req.Text)
	if err != nil || req.ExpectedRevision < 1 {
		return nil, ErrInvalidInput
	}
	var result response.MessageResponse
	err = s.transaction(ctx, actor, conversationID, func(tx ChatWrite) error {
		msg, err := tx.Messages.FindMessage(ctx, id)
		if err != nil {
			return err
		}
		if msg.SenderID == nil || *msg.SenderID != actor {
			return ErrForbidden
		}
		if msg.DeletedAt != nil || msg.Kind != enum.MessageKindText || msg.Revision != req.ExpectedRevision {
			return ErrConflict
		}
		if msg.Text != nil && *msg.Text == text {
			result = messageResponse(*msg)
			return nil
		}
		msg.Text, msg.EditedAt, msg.Revision = &text, &tx.Now, msg.Revision+1
		if err := tx.Messages.SaveMessage(ctx, msg); err != nil {
			return err
		}
		if err := tx.Events.AppendEvent(ctx, model.EventMessageUpdated, msg.ID, nil, messageEvent(msg)); err != nil {
			return err
		}
		result = messageResponse(*msg)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *MessageService) DeleteMessage(ctx context.Context, actor, conversationID, id uuid.UUID) error {
	return s.transaction(ctx, actor, conversationID, func(tx ChatWrite) error {
		msg, err := tx.Messages.FindMessage(ctx, id)
		if err != nil {
			return err
		}
		if (msg.SenderID == nil || *msg.SenderID != actor) && !tx.Member.Role.CanManageMembers() {
			return ErrForbidden
		}
		if msg.Kind != enum.MessageKindText {
			return ErrConflict
		}
		if msg.DeletedAt != nil {
			return nil
		}
		msg.Text, msg.DeletedAt, msg.Revision = nil, &tx.Now, msg.Revision+1
		if err := tx.Messages.SaveMessage(ctx, msg); err != nil {
			return err
		}
		return tx.Events.AppendEvent(ctx, model.EventMessageDeleted, msg.ID, nil, messageEvent(msg))
	})
}

func messageEvent(msg *model.Message) model.ConversationEventPayload {
	return model.ConversationEventPayload{MessageID: &msg.ID, MessageSeq: &msg.Seq, Revision: &msg.Revision}
}
