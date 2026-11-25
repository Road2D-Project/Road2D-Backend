package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedirectPublisher struct {
	client  *redis.Client
	ownerID uuid.UUID
	jobs    chan model.RedirectJob
}

func NewRedirectPublisher(client *redis.Client, ownerID uuid.UUID) *RedirectPublisher {
	return &RedirectPublisher{client: client, ownerID: ownerID, jobs: make(chan model.RedirectJob, 256)}
}

// TrySubmit does no network I/O on the room goroutine. A full queue leaves the
// redirect pending on the coordinator so a later tick can submit the same job.
func (p *RedirectPublisher) TrySubmit(job model.RedirectJob) bool {
	job.OwnerID = p.ownerID
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

func (p *RedirectPublisher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-p.jobs:
			for time.Now().Before(job.ExpiresAt) && ctx.Err() == nil {
				publishCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := p.publish(publishCtx, job)
				cancel()
				if err == nil {
					break
				}
				log.Printf("tracking publish job %s: %v", job.JobID, err)
				if !waitRetry(ctx) {
					return
				}
			}
		}
	}
}

func (p *RedirectPublisher) publish(ctx context.Context, job model.RedirectJob) error {
	payload, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: RedirectJobsStream,
		Values: map[string]interface{}{"job": string(payload)},
	}).Err()
}
