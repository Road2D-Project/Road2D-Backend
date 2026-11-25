package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/model/response"
	"Road-To-Destination-BE/module/chat/repository"
	"github.com/google/uuid"
)

type ConversationReader interface {
	ListConversations(context.Context, uuid.UUID, *repository.ConversationCursor, int) ([]repository.ConversationRow, error)
	GetConversation(context.Context, uuid.UUID, uuid.UUID) (*repository.ConversationRow, error)
	ListMessages(context.Context, uuid.UUID, uuid.UUID, request.ListMessagesRequest) ([]model.Message, error)
}

type ConversationService struct{ reads ConversationReader }

func NewConversationService(reads ConversationReader) *ConversationService {
	return &ConversationService{reads: reads}
}

func (s *ConversationService) ListConversations(ctx context.Context, actor uuid.UUID, query request.ListConversationsRequest) (*response.ConversationListResponse, error) {
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Limit < 1 || query.Limit > 50 || len(query.Cursor) > 512 {
		return nil, ErrInvalidQuery
	}
	var cursor *repository.ConversationCursor
	if query.Cursor != "" {
		cursor = &repository.ConversationCursor{}
		decoded, err := base64.RawURLEncoding.DecodeString(query.Cursor)
		if err != nil || json.Unmarshal(decoded, cursor) != nil || cursor.ID == uuid.Nil || cursor.ActivityAt.IsZero() {
			return nil, ErrInvalidQuery
		}
	}
	rows, err := s.reads.ListConversations(ctx, actor, cursor, query.Limit)
	if err != nil {
		return nil, err
	}
	out := &response.ConversationListResponse{Conversations: make([]response.ConversationResponse, 0, min(len(rows), query.Limit))}
	if len(rows) > query.Limit {
		rows = rows[:query.Limit]
		last := rows[len(rows)-1]
		encoded, err := json.Marshal(repository.ConversationCursor{ActivityAt: last.LastActivityAt, ID: last.ID})
		if err != nil {
			return nil, err
		}
		out.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	for _, row := range rows {
		out.Conversations = append(out.Conversations, conversationResponse(row))
	}
	return out, nil
}

func (s *ConversationService) GetConversation(ctx context.Context, actor, id uuid.UUID) (*response.ConversationResponse, error) {
	row, err := s.reads.GetConversation(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	out := conversationResponse(*row)
	return &out, nil
}

func (s *ConversationService) ListMessages(ctx context.Context, actor, id uuid.UUID, query request.ListMessagesRequest) (*response.MessageListResponse, error) {
	if query.Limit == 0 {
		query.Limit = 30
	}
	if query.Limit < 1 || query.Limit > 100 || (query.BeforeSeq != nil && query.AfterSeq != nil) ||
		(query.BeforeSeq != nil && *query.BeforeSeq <= 0) || (query.AfterSeq != nil && *query.AfterSeq < 0) {
		return nil, ErrInvalidQuery
	}
	rows, err := s.reads.ListMessages(ctx, actor, id, query)
	if err != nil {
		return nil, err
	}
	out := &response.MessageListResponse{Messages: make([]response.MessageResponse, 0, min(len(rows), query.Limit)), HasMore: len(rows) > query.Limit}
	if out.HasMore {
		rows = rows[:query.Limit]
		boundary := rows[len(rows)-1].Seq
		if query.AfterSeq != nil {
			out.NextAfterSeq = &boundary
		} else {
			out.NextBeforeSeq = &boundary
		}
	}
	if query.AfterSeq == nil {
		slices.Reverse(rows)
	}
	for _, row := range rows {
		out.Messages = append(out.Messages, messageResponse(row))
	}
	return out, nil
}

func conversationResponse(row repository.ConversationRow) response.ConversationResponse {
	out := response.ConversationResponse{ID: row.ID, TripID: row.TripID, Name: row.Name, MyRole: row.MyRole,
		ActiveMemberCount: row.ActiveMemberCount, UnreadCount: row.UnreadCount, LastReadSeq: row.LastReadSeq,
		LastMessageSeq: row.LastMessageSeq, LastMessageID: row.LastMessageID, LastMessageRevision: row.LastMessageRevision,
		LastActivityAt: row.LastActivityAt, SummaryVersion: row.SummaryVersion, MutedUntil: row.MutedUntil}
	if row.UnreadCount == 0 && row.LastMessageID != nil {
		out.LastMessagePreview = &response.MessagePreview{Kind: row.LastMessageKind, Text: row.LastMessageText, Deleted: row.LastMessageDeleted}
		if row.LastMessageDeleted {
			out.LastMessagePreview.Text = nil
		}
	}
	return out
}
