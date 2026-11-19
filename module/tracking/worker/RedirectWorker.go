package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"Road-To-Destination-BE/module/tracking/engine"
	"Road-To-Destination-BE/module/tracking/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type RedirectRouter interface {
	Redirect(context.Context, float64, float64, float64, float64) (string, float64, float64, error)
}

type RedirectResultStore interface {
	Find(context.Context, uuid.UUID) (*model.RedirectResult, error)
	Save(context.Context, model.RedirectResult) (*model.RedirectResult, error)
}

type RedirectWorker struct {
	client    *redis.Client
	router    RedirectRouter
	results   RedirectResultStore
	consumer  string
	claimAt   string
	claimIdle time.Duration
}

func NewRedirectWorker(client *redis.Client, router RedirectRouter, results RedirectResultStore) *RedirectWorker {
	return &RedirectWorker{
		client: client, router: router, results: results,
		consumer: uuid.NewString(), claimAt: "0-0", claimIdle: 30 * time.Second,
	}
}

func (w *RedirectWorker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		err := ensureGroup(ctx, w.client, RedirectJobsStream, redirectGroup)
		if err == nil {
			err = w.consume(ctx)
		}
		if err != nil && ctx.Err() == nil {
			log.Printf("tracking redirect worker: %v", err)
			if !waitRetry(ctx) {
				return
			}
		}
	}
}

func (w *RedirectWorker) consume(ctx context.Context) error {
	entries, cursor, err := w.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream: RedirectJobsStream, Group: redirectGroup, Consumer: w.consumer,
		MinIdle: w.claimIdle, Start: w.claimAt, Count: 1,
	}).Result()
	if err != nil {
		return err
	}
	w.claimAt = cursor
	if len(entries) == 0 {
		streams, err := w.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: redirectGroup, Consumer: w.consumer,
			Streams: []string{RedirectJobsStream, ">"}, Count: 1, Block: time.Second,
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
		// Finish or abandon the attempt before another worker can reclaim it.
		entryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := w.handle(entryCtx, entry)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *RedirectWorker) handle(ctx context.Context, entry redis.XMessage) error {
	var job model.RedirectJob
	raw, ok := entry.Values["job"].(string)
	if !ok {
		return rejectEntry(ctx, w.client, RedirectJobsStream, redirectGroup, entry, errors.New("missing redirect job"))
	}
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		return rejectEntry(ctx, w.client, RedirectJobsStream, redirectGroup, entry, err)
	}
	if job.JobID == uuid.Nil || job.OwnerID == uuid.Nil || job.TripID == uuid.Nil || job.MemberID == uuid.Nil ||
		job.SchemaVersion != model.RedirectSchemaVersion || job.ExpiresAt.IsZero() ||
		!validPoint(job.Origin) || !validPoint(job.Target) {
		return rejectEntry(ctx, w.client, RedirectJobsStream, redirectGroup, entry, errors.New("invalid redirect job"))
	}
	stored, err := w.results.Find(ctx, job.JobID)
	if err != nil {
		return err
	}
	if stored == nil {
		result := w.calculate(ctx, job)
		stored, err = w.results.Save(ctx, result)
		if err != nil {
			return err
		}
		if stored == nil {
			return errors.New("redirect result was not stored")
		}
	}
	payload, err := json.Marshal(stored)
	if err != nil {
		return err
	}
	stream := resultStream(stored.OwnerID)
	if err := w.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream, Values: map[string]interface{}{"result": string(payload)},
	}).Err(); err != nil {
		return err
	}
	if err := w.client.Expire(ctx, stream, resultRetention).Err(); err != nil {
		return err
	}
	// A crash before ACK reuses the durable result and republishes it. The room
	// accepts only the job ID and generation it is still waiting for.
	return acknowledge(ctx, w.client, RedirectJobsStream, redirectGroup, entry.ID)
}

func (w *RedirectWorker) calculate(ctx context.Context, job model.RedirectJob) model.RedirectResult {
	result := model.RedirectResult{
		JobID: job.JobID, OwnerID: job.OwnerID, TripID: job.TripID,
		MemberID: job.MemberID, Generation: job.Generation, SchemaVersion: job.SchemaVersion,
	}
	if !time.Now().Before(job.ExpiresAt) {
		result.Error = "redirect job expired"
		return result
	}
	deadline := minTime(job.ExpiresAt, time.Now().Add(engine.RedirectTimeout))
	callCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	polyline, distance, duration, err := w.router.Redirect(callCtx, job.Origin.Lat, job.Origin.Lng, job.Target.Lat, job.Target.Lng)
	if err == nil {
		var points []model.Point
		points, err = model.DecodePolyline(polyline)
		if err == nil && (len(points) < 2 || math.IsNaN(distance) || math.IsInf(distance, 0) || distance < 0 ||
			math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 0) {
			err = errors.New("invalid redirect route")
		}
	}
	if err != nil {
		result.Error = fmt.Sprintf("compute redirect: %v", err)
		return result
	}
	result.Polyline, result.DistanceM, result.DurationS = polyline, distance, duration
	return result
}

func validPoint(point model.Point) bool {
	return !math.IsNaN(point.Lat) && !math.IsNaN(point.Lng) &&
		point.Lat >= -90 && point.Lat <= 90 && point.Lng >= -180 && point.Lng <= 180
}

func minTime(left, right time.Time) time.Time {
	if left.Before(right) {
		return left
	}
	return right
}
