package chatting

import (
	"context"
	"time"

	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

type AccessReader interface {
	Recipients(context.Context, uuid.UUID) ([]uuid.UUID, error)
	ConversationForTrip(context.Context, uuid.UUID) (uuid.UUID, error)
}

type AccessLifecycle struct {
	hubs  *ChattingHubs
	reads AccessReader
}

func NewAccessLifecycle(hubs *ChattingHubs, reads AccessReader) *AccessLifecycle {
	return &AccessLifecycle{hubs: hubs, reads: reads}
}

func (l *AccessLifecycle) TripChange(ctx context.Context, tripID uuid.UUID, run func(context.Context) error) error {
	return l.hubs.WithAccess(ctx, func() error {
		id, err := l.reads.ConversationForTrip(ctx, tripID)
		if err != nil {
			return err
		}
		before, err := l.reads.Recipients(ctx, id)
		if err != nil {
			return err
		}
		if err := run(ctx); err != nil {
			return err
		}
		// Request cancellation cannot skip post-commit revocation.
		revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		after, err := l.reads.Recipients(revoke, id)
		if err != nil {
			// The mutation committed. Closing the entire room is sufficient for
			// local revocation; the durable event retries recipient reads later.
			force, stop := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
			defer stop()
			_, closeErr := l.hubs.Conversations.CloseRoom(force, id.String())
			return closeErr
		}
		active := make(map[uuid.UUID]bool, len(after))
		for _, user := range after {
			active[user] = true
		}
		for _, user := range before {
			if !active[user] {
				if err := l.evictMember(revoke, id, user); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (l *AccessLifecycle) UserChange(ctx context.Context, userID uuid.UUID, sessionID string, beforeVersion int64, run func(context.Context) error) error {
	return l.hubs.WithAccess(ctx, func() error {
		if err := run(ctx); err != nil {
			return err
		}
		revoke, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return l.evictSession(revoke, userID, sessionID, beforeVersion)
	})
}

func (l *AccessLifecycle) evictMember(ctx context.Context, conversationID, userID uuid.UUID) error {
	if _, err := l.hubs.Conversations.EvictUser(ctx, conversationID.String(), userID); err != nil {
		force, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_, err = l.hubs.Conversations.CloseRoom(force, conversationID.String())
		return err
	}
	return nil
}

func (l *AccessLifecycle) evictSession(ctx context.Context, userID uuid.UUID, sessionID string, beforeVersion int64) error {
	filter := realtime.SessionFilter{SessionID: sessionID, BeforeVersion: beforeVersion}
	for _, hub := range []*realtime.Hub{l.hubs.Conversations, l.hubs.Personal} {
		if err := hub.EvictSession(ctx, userID, filter); err != nil {
			return err
		}
	}
	return nil
}
