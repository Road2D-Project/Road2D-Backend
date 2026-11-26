package controller_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"Road-To-Destination-BE/middleware"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	authService "Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/conversation/controller"
	conversationModel "Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/conversation/model/response"
	"Road-To-Destination-BE/module/share/outbox"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestReplayExpiryAndInternalJobFiltering(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	path := "/conversations/" + c.ID.String()
	f.send(t, &f.owner, "POST", path+"/messages", messageBody("retained text"), 201)
	seq := int64(2)
	internal := outbox.Event{Type: "media.delete", AggregateID: c.ID, ConversationID: &c.ID,
		EventSeq: &seq, Payload: json.RawMessage(`{"path":"private-storage-key"}`)}
	if err := f.db.Create(&internal).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.db.Model(&c).Update("next_event_seq", 3).Error; err != nil {
		t.Fatal(err)
	}
	w := f.get(t, &f.owner, path+"/events", 200)
	page := decode[response.EventListResponse](t, w)
	if len(page.Events) != 1 || page.NextEventSeq != 2 || strings.Contains(w.Body.String(), "private-storage-key") {
		t.Fatalf("internal event leaked or cursor stuck: %s", w.Body.String())
	}
	if err := f.db.Model(&outbox.Event{}).Where("conversation_id = ?", c.ID).Update("created_at", time.Now().Add(-8*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	expired := decode[response.ReplayRequiredResponse](t, f.get(t, &f.owner, path+"/events", 410))
	if !expired.SyncRequired || expired.MinAfterEventSeq != 2 || expired.LatestEventSeq != 2 {
		t.Fatalf("expired bounds: %+v", expired)
	}
	empty := decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events?afterEventSeq=2", 200))
	if len(empty.Events) != 0 || empty.Events == nil || empty.NextEventSeq != 2 {
		t.Fatalf("up-to-date replay: %+v", empty)
	}
	f.get(t, &f.outsider, path+"/events", 404)
	// A new retained event permits replay after the expired prefix, not before it.
	f.send(t, &f.owner, "POST", path+"/messages", messageBody("new retained event"), 201)
	// Simulate timestamp order differing from sequence order after a clock change.
	if err := f.db.Model(&outbox.Event{}).Where("conversation_id = ? AND event_seq = 1", c.ID).Update("created_at", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	f.get(t, &f.owner, path+"/events?afterEventSeq=1", 410)
	latest := decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events?afterEventSeq=2", 200))
	if len(latest.Events) != 1 || latest.Events[0].EventSeq != 3 {
		t.Fatalf("new replay: %+v", latest)
	}
}

func TestReadRacingMessageLeavesNewArrivalUnread(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	path := "/conversations/" + c.ID.String()
	f.send(t, &f.owner, "POST", path+"/messages", messageBody("first"), 201)
	ownerToken, err := authService.CreateToken(&f.owner)
	if err != nil {
		t.Fatal(err)
	}
	memberToken, err := authService.CreateToken(&f.member)
	if err != nil {
		t.Fatal(err)
	}
	newBody, _ := json.Marshal(messageBody("second"))
	operations := []struct {
		method, suffix, token string
		body                  []byte
		status                int
	}{
		{"POST", "/messages", ownerToken, newBody, 201},
		{"PUT", "/read", memberToken, []byte(`{"lastReadSeq":1}`), 200},
	}
	results := make([]*httptest.ResponseRecorder, len(operations))
	var workers sync.WaitGroup
	for i, op := range operations {
		workers.Add(1)
		go func(i int) {
			defer workers.Done()
			req := httptest.NewRequest(op.method, "/v1"+path+op.suffix, bytes.NewReader(op.body))
			req.Header.Set("Authorization", "Bearer "+op.token)
			results[i] = httptest.NewRecorder()
			f.router.ServeHTTP(results[i], req)
		}(i)
	}
	workers.Wait()
	for i, w := range results {
		if w.Code != operations[i].status {
			t.Fatalf("race operation failed: %d %s", w.Code, w.Body.String())
		}
	}
	summary := decode[response.ConversationResponse](t, f.get(t, &f.member, path, 200))
	if summary.LastReadSeq != 1 || summary.LastMessageSeq != 2 || summary.UnreadCount != 1 || summary.SummaryVersion != 3 {
		t.Fatalf("new arrival incorrectly marked read: %+v", summary)
	}
	if summary.LastMessagePreview != nil {
		t.Fatal("unread preview exposed")
	}
	// Deleting the unread arrival reduces the count without moving the cursor.
	history := decode[response.MessageListResponse](t, f.get(t, &f.owner, path+"/messages?afterSeq=1", 200))
	f.send(t, &f.owner, "DELETE", path+"/messages/"+history.Messages[0].ID.String(), nil, 204)
	summary = decode[response.ConversationResponse](t, f.get(t, &f.member, path, 200))
	if summary.UnreadCount != 0 || summary.LastReadSeq != 1 || summary.LastMessagePreview == nil || !summary.LastMessagePreview.Deleted {
		t.Fatalf("unread deletion summary: %+v", summary)
	}
}

func TestMessageSendLimiterIsPerUserAcrossConversations(t *testing.T) {
	f := chatDatabase(t)
	first, second := f.trip(t), f.trip(t)
	cache := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: cache.Addr()})
	t.Cleanup(func() { client.Close() })
	limiter, err := middleware.NewTokenBucket(middleware.TokenBucketConfig{Capacity: 1, RefillRate: 1, RefillInterval: time.Hour}, client)
	if err != nil {
		t.Fatal(err)
	}
	auth := middleware.NewAuthenticationMiddleware(authRepo.NewUserRepository(f.db))
	controller.NewChatController(f.db, auth, limiter).RegisterRoutes(f.router.Group("/v1/limited"))
	f.send(t, &f.owner, "POST", "/limited/conversations/"+first.ID.String()+"/messages", messageBody("allowed"), 201)
	f.send(t, &f.owner, "POST", "/limited/conversations/"+second.ID.String()+"/messages", messageBody("limited"), 429)
	f.send(t, &f.member, "POST", "/limited/conversations/"+second.ID.String()+"/messages", messageBody("different user"), 201)
	var messages int64
	if err := f.db.Model(&conversationModel.Message{}).Count(&messages).Error; err != nil || messages != 2 {
		t.Fatalf("limiter side effects: count=%d err=%v", messages, err)
	}
}
