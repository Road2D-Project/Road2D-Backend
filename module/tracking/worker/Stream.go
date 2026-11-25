package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	RedirectJobsStream = "road2d:tracking:redirect:jobs"
	redirectGroup      = "redirect-workers"
	resultGroup        = "tracking-owner"
	resultRetention    = 24 * time.Hour
	streamRetry        = time.Second
)

func resultStream(ownerID uuid.UUID) string {
	return "road2d:tracking:redirect:results:" + ownerID.String()
}

func ensureGroup(ctx context.Context, client *redis.Client, stream, group string) error {
	err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err()
	if redis.HasErrorPrefix(err, "BUSYGROUP") {
		return nil
	}
	return err
}

// Each of these streams has one consumer group. Removing an acknowledged entry
// keeps stream storage bounded without trimming work that is still pending.
var acknowledgeEntry = redis.NewScript(`
redis.call('XACK', KEYS[1], ARGV[1], ARGV[2])
return redis.call('XDEL', KEYS[1], ARGV[2])
`)

func acknowledge(ctx context.Context, client *redis.Client, stream, group, id string) error {
	return acknowledgeEntry.Run(ctx, client, []string{stream}, group, id).Err()
}

func rejectEntry(ctx context.Context, client *redis.Client, stream, group string, entry redis.XMessage, cause error) error {
	payload, err := json.Marshal(entry.Values)
	if err != nil {
		return err
	}
	err = client.XAdd(ctx, &redis.XAddArgs{
		Stream: "road2d:tracking:dead", MaxLen: 10000, Approx: true,
		Values: map[string]interface{}{
			"source": stream, "entryId": entry.ID,
			"error": cause.Error(), "payload": string(payload),
		},
	}).Err()
	if err != nil {
		return err
	}
	return acknowledge(ctx, client, stream, group, entry.ID)
}

func waitRetry(ctx context.Context) bool {
	timer := time.NewTimer(streamRetry)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
