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
	access        chan struct{}
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
	return &ChattingHubs{Conversations: conversations, Personal: personal, access: make(chan struct{}, 1)}, nil
}

// WithAccess serializes admission, prepared delivery and access-changing commits
// in this single-owner deployment. Room callbacks never acquire this gate.
func (h *ChattingHubs) WithAccess(ctx context.Context, run func() error) error {
	select {
	case h.access <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-h.access }()
	if err := ctx.Err(); err != nil {
		return err
	}
	return run()
}

func (h *ChattingHubs) Close(ctx context.Context) error {
	return errors.Join(h.Conversations.Close(ctx), h.Personal.Close(ctx))
}
