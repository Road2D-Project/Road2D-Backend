package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"Road-To-Destination-BE/module/tracking/model"
	"Road-To-Destination-BE/utils/polyline"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type memoryResults struct {
	mu      sync.Mutex
	rows    map[uuid.UUID]model.RedirectResult
	saveErr error
}

func (s *memoryResults) Find(_ context.Context, id uuid.UUID) (*model.RedirectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.rows[id]
	if !ok {
		return nil, nil
	}
	return &result, nil
}

func (s *memoryResults) Save(_ context.Context, result model.RedirectResult) (*model.RedirectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveErr != nil {
		return nil, s.saveErr
	}
	if existing, ok := s.rows[result.JobID]; ok {
		return &existing, nil
	}
	s.rows[result.JobID] = result
	return &result, nil
}

type testRedirectRouter struct{ calls atomic.Int32 }

func (r *testRedirectRouter) Redirect(context.Context, float64, float64, float64, float64) (string, float64, float64, error) {
	r.calls.Add(1)
	return polyline.Encode([]polyline.Point{{Lat: 10, Lng: 106}, {Lat: 11, Lng: 106}}), 100, 10, nil
}

func workerFixture(t *testing.T) (*redis.Client, *RedirectWorker, *memoryResults, *testRedirectRouter, model.RedirectJob) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := &memoryResults{rows: make(map[uuid.UUID]model.RedirectResult)}
	router := &testRedirectRouter{}
	worker := NewRedirectWorker(client, router, store)
	if err := ensureGroup(context.Background(), client, RedirectJobsStream, redirectGroup); err != nil {
		t.Fatal(err)
	}
	job := model.RedirectJob{
		JobID: uuid.New(), OwnerID: uuid.New(), TripID: uuid.New(), MemberID: uuid.New(),
		Generation: 4, SchemaVersion: model.RedirectSchemaVersion,
		Origin: model.Point{Lat: 10, Lng: 106}, Target: model.Point{Lat: 11, Lng: 106},
		ExpiresAt: time.Now().Add(time.Minute),
	}
	return client, worker, store, router, job
}

func enqueueJob(t *testing.T, client *redis.Client, job model.RedirectJob) {
	t.Helper()
	if err := NewRedirectPublisher(client, job.OwnerID).publish(context.Background(), job); err != nil {
		t.Fatal(err)
	}
}

func TestRedirectPipelineReusesStoredResultAndRoutesToOwner(t *testing.T) {
	client, worker, _, router, job := workerFixture(t)
	ctx := context.Background()
	consumer := NewRedirectResultConsumer(client, job.OwnerID)
	inbox, unsubscribe := consumer.Subscribe(job.TripID)
	defer unsubscribe()
	stream := resultStream(job.OwnerID)
	if err := ensureGroup(ctx, client, stream, resultGroup); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		enqueueJob(t, client, job)
		if err := worker.consume(ctx); err != nil {
			t.Fatal(err)
		}
		if err := consumer.consume(ctx, stream); err != nil {
			t.Fatal(err)
		}
		select {
		case result := <-inbox:
			if result.JobID != job.JobID || result.Generation != job.Generation || result.Polyline == "" {
				t.Fatalf("unexpected result %+v", result)
			}
		default:
			t.Fatal("owner did not receive result")
		}
	}
	if router.calls.Load() != 1 {
		t.Fatalf("provider called %d times for one durable job", router.calls.Load())
	}
	if count := client.XLen(ctx, RedirectJobsStream).Val(); count != 0 {
		t.Fatalf("%d completed jobs retained", count)
	}
	if count := client.XLen(ctx, stream).Val(); count != 0 {
		t.Fatalf("%d delivered results retained", count)
	}
}

func TestRedirectWorkerClaimsPendingJobAfterConsumerCrash(t *testing.T) {
	client, worker, _, router, job := workerFixture(t)
	ctx := context.Background()
	enqueueJob(t, client, job)
	if err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: redirectGroup, Consumer: "crashed-worker",
		Streams: []string{RedirectJobsStream, ">"}, Count: 1,
	}).Err(); err != nil {
		t.Fatal(err)
	}
	worker.claimIdle = 0
	if err := worker.consume(ctx); err != nil {
		t.Fatal(err)
	}
	if router.calls.Load() != 1 || client.XPending(ctx, RedirectJobsStream, redirectGroup).Val().Count != 0 {
		t.Fatal("pending work was not recovered and acknowledged")
	}
}

func TestRedirectWorkerKeepsJobPendingUntilResultIsStored(t *testing.T) {
	client, worker, store, _, job := workerFixture(t)
	ctx := context.Background()
	store.saveErr = errors.New("database down")
	enqueueJob(t, client, job)
	if err := worker.consume(ctx); !errors.Is(err, store.saveErr) {
		t.Fatalf("got %v", err)
	}
	if count := client.XPending(ctx, RedirectJobsStream, redirectGroup).Val().Count; count != 1 {
		t.Fatalf("pending = %d, want 1", count)
	}
	if count := client.XLen(ctx, resultStream(job.OwnerID)).Val(); count != 0 {
		t.Fatal("result published before persistence")
	}
}

func TestExpiredRedirectJobDoesNotCallProvider(t *testing.T) {
	client, worker, store, router, job := workerFixture(t)
	job.ExpiresAt = time.Now().Add(-time.Second)
	enqueueJob(t, client, job)
	if err := worker.consume(context.Background()); err != nil {
		t.Fatal(err)
	}
	result, err := store.Find(context.Background(), job.JobID)
	if err != nil || result == nil || result.Error == "" || router.calls.Load() != 0 {
		t.Fatalf("result %+v, error %v, calls %d", result, err, router.calls.Load())
	}
}

func TestInvalidJobGoesToDeadLetterAndDoesNotBlockStream(t *testing.T) {
	client, worker, _, router, _ := workerFixture(t)
	ctx := context.Background()
	if err := client.XAdd(ctx, &redis.XAddArgs{
		Stream: RedirectJobsStream, Values: map[string]interface{}{"job": "invalid-json"},
	}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := worker.consume(ctx); err != nil {
		t.Fatal(err)
	}
	if client.XLen(ctx, "road2d:tracking:dead").Val() != 1 || router.calls.Load() != 0 {
		t.Fatal("invalid job was not quarantined")
	}
}

func TestFullResultInboxLeavesEntryPending(t *testing.T) {
	client, _, _, _, job := workerFixture(t)
	ctx := context.Background()
	consumer := NewRedirectResultConsumer(client, job.OwnerID)
	inbox, unsubscribe := consumer.Subscribe(job.TripID)
	defer unsubscribe()
	for range cap(inbox) {
		consumer.rooms[job.TripID] <- model.RedirectResult{}
	}
	stream := resultStream(job.OwnerID)
	if err := ensureGroup(ctx, client, stream, resultGroup); err != nil {
		t.Fatal(err)
	}
	result := model.RedirectResult{
		JobID: job.JobID, OwnerID: job.OwnerID, TripID: job.TripID,
		SchemaVersion: model.RedirectSchemaVersion,
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: map[string]interface{}{"result": string(payload)}}).Err(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.consume(ctx, stream); err != nil {
		t.Fatal(err)
	}
	if client.XPending(ctx, stream, resultGroup).Val().Count != 1 {
		t.Fatal("full inbox discarded result")
	}
}
