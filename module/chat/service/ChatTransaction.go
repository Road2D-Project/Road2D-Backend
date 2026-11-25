package service

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/repository"
	tripModel "Road-To-Destination-BE/module/trip/model"
	"github.com/google/uuid"
)

type MessageStore interface {
	FindClientMessage(context.Context, uuid.UUID) (*model.Message, error)
	FindMessage(context.Context, uuid.UUID) (*model.Message, error)
	InsertMessage(context.Context, *model.Message) error
	SaveMessage(context.Context, *model.Message) error
}

type UserStateStore interface {
	GetUserState(context.Context) (*model.ConversationUserState, error)
	SaveUserState(context.Context, *model.ConversationUserState) error
	Summary(context.Context) (*repository.ConversationRow, error)
}

type ConversationEventAppender interface {
	AppendEvent(context.Context, string, uuid.UUID, *uuid.UUID, model.ConversationEventPayload) error
}

type ChatWrite struct {
	Conversation *model.Conversation
	Member       *tripModel.TripMember
	Now          time.Time
	Messages     MessageStore
	State        UserStateStore
	Events       ConversationEventAppender
}

// ChatTransaction supplies capabilities bound to one database transaction.
type ChatTransaction func(context.Context, uuid.UUID, uuid.UUID, func(ChatWrite) error) error
