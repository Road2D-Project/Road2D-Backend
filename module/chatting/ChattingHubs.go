package chatting

import (
	"context"
	"errors"
	"time"

	"Road-To-Destination-BE/module/realtime"
)

// ChattingHubs wires the conversation and personal channels. Socket runtime
// belongs to realtime; domain callbacks belong to the per-room coordinators.
type ChattingHubs struct {
	Conversations *realtime.Hub
	Personal      *realtime.Hub
}

func NewChattingHubs(idleTimeout time.Duration) (*ChattingHubs, error) {
	options := realtime.DefaultRoomOptions()
	options.IdleTimeout, options.TickInterval = idleTimeout, 0
	conversations, err := realtime.NewHubWithOptions(options)
	if err != nil {
		return nil, err
	}
	personal, err := realtime.NewHubWithOptions(options)
	if err != nil {
		return nil, err
	}
	if err := conversations.Mount("chat", NewChattingModule()); err != nil {
		return nil, err
	}
	if err := personal.Mount("chat", NewPersonalModule()); err != nil {
		return nil, err
	}
	return &ChattingHubs{Conversations: conversations, Personal: personal}, nil
}

func (h *ChattingHubs) Close(ctx context.Context) error {
	return errors.Join(h.Conversations.Close(ctx), h.Personal.Close(ctx))
}
