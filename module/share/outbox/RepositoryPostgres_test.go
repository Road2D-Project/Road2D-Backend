package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"Road-To-Destination-BE/internal/testutil"
	authModel "Road-To-Destination-BE/module/authentication/model"
	conversation "Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/conversation/model/request"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/share/outbox"
	tripModel "Road-To-Destination-BE/module/trip/model"
	tripRepository "Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestClaimsSerializeConversationRetriesAndFenceExpiredLeases(t *testing.T) {
	db, scope, _ := outboxDatabase(t)
	first := seedEvent(t, db, scope.ID, 1, time.Now(), false)
	second := seedEvent(t, db, scope.ID, 2, time.Now(), false)
	repo := outbox.NewRepository(db)
	ctx := context.Background()
	var claims [2][]outbox.Event
	var claimErrors [2]error
	var workers sync.WaitGroup
	for i := range claims {
		workers.Add(1)
		go func(i int) { defer workers.Done(); claims[i], claimErrors[i] = repo.Claim(ctx, 32, time.Minute) }(i)
	}
	workers.Wait()
	var claimed []outbox.Event
	for i := range claims {
		if claimErrors[i] != nil {
			t.Fatal(claimErrors[i])
		}
		claimed = append(claimed, claims[i]...)
	}
	if len(claimed) != 1 || claimed[0].ID != first.ID {
		t.Fatalf("competing claims = %#v", claimed)
	}
	if err := repo.Retry(ctx, claimed[0]); err != nil {
		t.Fatal(err)
	}
	if pending, err := repo.Claim(ctx, 32, time.Minute); err != nil || len(pending) != 0 {
		t.Fatalf("later event passed a backing-off predecessor: %#v %v", pending, err)
	}
	if err := db.Model(&outbox.Event{}).Where("id = ?", first.ID).Update("next_attempt_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	retry, err := repo.Claim(ctx, 32, time.Minute)
	if err != nil || len(retry) != 1 || retry[0].Attempts != 2 {
		t.Fatalf("retry claim = %#v %v", retry, err)
	}
	if err := db.Model(&outbox.Event{}).Where("id = ?", first.ID).Update("claimed_until", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	reclaimed, err := repo.Claim(ctx, 32, time.Minute)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].Attempts != 3 {
		t.Fatalf("expired lease = %#v %v", reclaimed, err)
	}
	if err := repo.Complete(ctx, retry[0]); err != nil {
		t.Fatal(err)
	}
	var row outbox.Event
	if err := db.First(&row, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if row.DispatchedAt != nil {
		t.Fatal("a stale lease completed another claim")
	}
	if err := repo.Complete(ctx, reclaimed[0]); err != nil {
		t.Fatal(err)
	}
	next, err := repo.Claim(ctx, 32, time.Minute)
	if err != nil || len(next) != 1 || next[0].ID != second.ID {
		t.Fatalf("successor = %#v %v", next, err)
	}
}

func TestRetentionPersistsFloorAndNeverDeletesOldPendingWork(t *testing.T) {
	db, scope, owner := outboxDatabase(t)
	seedEvent(t, db, scope.ID, 1, time.Now(), true)
	expired := seedEvent(t, db, scope.ID, 2, time.Now().Add(-8*24*time.Hour), true)
	seedEvent(t, db, scope.ID, 3, time.Now(), true)
	pending := outbox.Event{Type: outbox.SessionRevoked, AggregateID: owner.ID, Payload: json.RawMessage(`{"beforeVersion":2}`)}
	pending.CreatedAt = time.Now().Add(-30 * 24 * time.Hour)
	if err := db.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := outbox.NewRepository(db).Prune(t.Context(), 1); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Model(&outbox.Event{}).Where("id = ?", expired.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("expired row remained: %d %v", count, err)
	}
	if err := db.Model(&outbox.Event{}).Where("id = ?", pending.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("pending row was deleted: %d %v", count, err)
	}
	var fresh conversation.Conversation
	if err := db.First(&fresh, "id = ?", scope.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fresh.ReplayFloor != 2 {
		t.Fatalf("floor = %d", fresh.ReplayFloor)
	}
	// Earlier retained rows can have newer timestamps after a clock adjustment.
	// Once the later expired row is gone, only the persistent floor preserves 410.
	_, err := conversationRepository.NewConversationRepository(db).ListEvents(t.Context(), owner.ID, scope.ID, request.ListEventsRequest{AfterEventSeq: 1, Limit: 10})
	var required *conversationRepository.ReplayExpiredError
	if !errors.As(err, &required) || required.MinAfterEventSeq != 2 {
		t.Fatalf("purged cursor did not expire: %v", err)
	}
	retained, err := conversationRepository.NewConversationRepository(db).ListEvents(t.Context(), owner.ID, scope.ID, request.ListEventsRequest{AfterEventSeq: 2, Limit: 10})
	if err != nil || len(retained.Events) != 1 || retained.Events[0].EventSeq != 3 {
		t.Fatalf("retained replay = %#v %v", retained, err)
	}
}

func outboxDatabase(t *testing.T) (*gorm.DB, conversation.Conversation, authModel.User) {
	t.Helper()
	db := testutil.Postgres(t)
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	user := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test", SessionVersion: 1}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	trip := &tripModel.Trip{OwnerID: user.ID, Name: "Outbox", Status: enum.TripPlanning, MemberLimit: 5, InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	now := time.Now()
	if err := tripRepository.NewTripRepository(db).CreateTripWithMembers(t.Context(), trip, []tripModel.TripMember{{UserID: user.ID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: &now}}, nil); err != nil {
		t.Fatal(err)
	}
	var scope conversation.Conversation
	if err := db.First(&scope, "trip_id = ?", trip.ID).Error; err != nil {
		t.Fatal(err)
	}
	return db, scope, user
}

func seedEvent(t *testing.T, db *gorm.DB, id uuid.UUID, seq int64, created time.Time, dispatched bool) outbox.Event {
	t.Helper()
	if err := db.Model(&conversation.Conversation{}).Where("id = ?", id).Update("next_event_seq", seq+1).Error; err != nil {
		t.Fatal(err)
	}
	event := outbox.Event{Type: conversation.EventMessageCreated, AggregateID: uuid.New(), ConversationID: &id, EventSeq: &seq, Payload: json.RawMessage(`{}`)}
	event.CreatedAt = created
	if dispatched {
		now := time.Now()
		event.DispatchedAt = &now
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	return event
}
