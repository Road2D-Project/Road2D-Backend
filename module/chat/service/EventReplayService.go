package service

import (
	"Road-To-Destination-BE/module/chat/model/request"
	"Road-To-Destination-BE/module/chat/model/response"
	"context"
	"github.com/google/uuid"
)

type EventReplayReader interface {
	ListEvents(context.Context, uuid.UUID, uuid.UUID, request.ListEventsRequest) (*response.EventListResponse, error)
}

type EventReplayService struct{ reads EventReplayReader }

func NewEventReplayService(reads EventReplayReader) *EventReplayService {
	return &EventReplayService{reads: reads}
}

func (s *EventReplayService) ListEvents(ctx context.Context, actor, id uuid.UUID, query request.ListEventsRequest) (*response.EventListResponse, error) {
	if query.Limit == 0 {
		query.Limit = 100
	}
	if query.AfterEventSeq < 0 || query.Limit < 1 || query.Limit > 200 {
		return nil, ErrInvalidQuery
	}
	return s.reads.ListEvents(ctx, actor, id, query)
}
