package worker

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"sync"
	"time"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedirectResultConsumer belongs to one server process. Only that owner's stream
// is consumed here, so another server cannot steal results for its rooms.
type RedirectResultConsumer struct {
	client  *redis.Client
	ownerID uuid.UUID
	mu      sync.Mutex
	rooms   map[uuid.UUID]chan model.RedirectResult
	claimAt string
}

func NewRedirectResultConsumer(client *redis.Client, ownerID uuid.UUID) *RedirectResultConsumer {
	return &RedirectResultConsumer{
		client: client, ownerID: ownerID,
		rooms: make(map[uuid.UUID]chan model.RedirectResult), claimAt: "0-0",
	}
}

// Subscribe is called on the first join, after the hub selects the room factory
// winner. Channels stay open so a consumer can never send to a closed channel.
func (c *RedirectResultConsumer) Subscribe(tripID uuid.UUID) (<-chan model.RedirectResult, func()) {
	inbox := make(chan model.RedirectResult, 64)
	c.mu.Lock()
	c.rooms[tripID] = inbox
	c.mu.Unlock()
	return inbox, func() {
		c.mu.Lock()
		if c.rooms[tripID] == inbox {
			delete(c.rooms, tripID)
		}
		c.mu.Unlock()
	}
}

func (c *RedirectResultConsumer) Run(ctx context.Context) {
	stream := resultStream(c.ownerID)
	for ctx.Err() == nil {
		err := ensureGroup(ctx, c.client, stream, resultGroup)
		if err == nil {
			err = c.client.Expire(ctx, stream, resultRetention).Err()
		}
		if err == nil {
			err = c.consume(ctx, stream)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("tracking redirect results: %v", err)
			if !waitRetry(ctx) {
				return
			}
		}
	}
}

func (c *RedirectResultConsumer) consume(ctx context.Context, stream string) error {
	entries, cursor, err := c.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: stream, Group: resultGroup, Consumer: c.ownerID.String(),
		MinIdle: time.Second, Start: c.claimAt, Count: 64,
	}).Result()
	if err != nil {
		return err
	}
	c.claimAt = cursor
	if len(entries) == 0 {
		streams, err := c.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: resultGroup, Consumer: c.ownerID.String(),
			Streams: []string{stream, ">"}, Count: 64, Block: time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) {
			return nil
		}
		if err != nil {
			return err
		}
		entries = streams[0].Messages
	}
	for _, entry := range entries {
		if err := c.handle(ctx, stream, entry); err != nil {
			return err
		}
	}
	return nil
}

func (c *RedirectResultConsumer) handle(ctx context.Context, stream string, entry redis.XMessage) error {
	var result model.RedirectResult
	raw, ok := entry.Values["result"].(string)
	if !ok {
		return rejectEntry(ctx, c.client, stream, resultGroup, entry, errors.New("missing redirect result"))
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return rejectEntry(ctx, c.client, stream, resultGroup, entry, err)
	}
	if result.OwnerID != c.ownerID || result.SchemaVersion != model.RedirectSchemaVersion || result.JobID == uuid.Nil {
		return rejectEntry(ctx, c.client, stream, resultGroup, entry, errors.New("invalid redirect result"))
	}
	c.mu.Lock()
	inbox := c.rooms[result.TripID]
	c.mu.Unlock()
	if inbox != nil {
		select {
		case inbox <- result:
		default:
			// The entry remains pending until the room drains its inbox.
			return nil
		}
	}
	// A closed room has no authoritative in-memory request left. Its successor
	// restores the checkpoint and issues a fresh job after a fresh GPS sample.
	return acknowledge(ctx, c.client, stream, resultGroup, entry.ID)
}
