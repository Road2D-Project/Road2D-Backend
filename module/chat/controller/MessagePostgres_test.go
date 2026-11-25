package controller_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	authModel "Road-To-Destination-BE/module/authentication/model"
	authService "Road-To-Destination-BE/module/authentication/service"
	chatModel "Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/chat/model/response"
	"Road-To-Destination-BE/module/share/outbox"
	tripModel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (f *chatFixture) send(t *testing.T, user *authModel.User, method, path string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, "/v1"+path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	if user != nil {
		token, err := authService.CreateToken(user)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}

func messageBody(text string) map[string]any {
	return map[string]any{"clientMessageId": uuid.NewString(), "text": text}
}

func TestMessageMutationsAndPrivateReplay(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	path := "/conversations/" + c.ID.String()
	body := messageBody("  Hello member  ")
	w := f.send(t, &f.owner, "POST", path+"/messages", body, 201)
	first := decode[response.MessageResponse](t, w)
	if first.Seq != 1 || first.Revision != 1 || first.Text == nil || *first.Text != "Hello member" {
		t.Fatalf("created message: %+v", first)
	}
	retry := decode[response.MessageResponse](t, f.send(t, &f.owner, "POST", path+"/messages", body, 200))
	if retry.ID != first.ID {
		t.Fatal("retry inserted a second message")
	}
	body["text"] = "different body"
	f.send(t, &f.owner, "POST", path+"/messages", body, 409)
	memberSummary := decode[response.ConversationResponse](t, f.get(t, &f.member, path, 200))
	if memberSummary.UnreadCount != 1 || memberSummary.LastReadSeq != 0 {
		t.Fatalf("incoming summary: %+v", memberSummary)
	}
	read := decode[response.ConversationResponse](t, f.send(t, &f.member, "PUT", path+"/read", map[string]any{"lastReadSeq": 1}, 200))
	if read.UnreadCount != 0 || read.LastReadSeq != 1 || read.SummaryVersion != 2 {
		t.Fatalf("read ack: %+v", read)
	}
	staleRead := decode[response.ConversationResponse](t, f.send(t, &f.member, "PUT", path+"/read", map[string]any{"lastReadSeq": 0}, 200))
	if staleRead.LastReadSeq != 1 || staleRead.SummaryVersion != 2 {
		t.Fatal("stale read regressed cursor or emitted an event")
	}
	until := time.Now().UTC().Add(time.Hour).Format(time.RFC3339Nano)
	pref := map[string]any{"mutedUntil": until}
	muted := decode[response.ConversationResponse](t, f.send(t, &f.member, "PATCH", path+"/preferences", pref, 200))
	if muted.MutedUntil == nil || muted.LastReadSeq != 1 || muted.SummaryVersion != 3 {
		t.Fatalf("mute: %+v", muted)
	}
	repeatedMute := decode[response.ConversationResponse](t, f.send(t, &f.member, "PATCH", path+"/preferences", pref, 200))
	if repeatedMute.SummaryVersion != 3 {
		t.Fatal("same timestamp preference is not idempotent")
	}
	privateTail := decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events?afterEventSeq=1", 200))
	if len(privateTail.Events) != 0 || privateTail.NextEventSeq != 3 || privateTail.HasMore {
		t.Fatalf("private-only tail cursor: %+v", privateTail)
	}
	messagePath := path + "/messages/" + first.ID.String()
	f.send(t, &f.member, "PATCH", messagePath, map[string]any{"text": "edited", "expectedRevision": 1}, 403)
	f.send(t, &f.member, "DELETE", messagePath, nil, 403)
	updated := decode[response.MessageResponse](t, f.send(t, &f.owner, "PATCH", messagePath, map[string]any{"text": "edited", "expectedRevision": 1}, 200))
	if updated.ID != first.ID || updated.Seq != first.Seq || updated.Revision != 2 {
		t.Fatalf("edit changed message identity: %+v", updated)
	}
	f.send(t, &f.owner, "PATCH", messagePath, map[string]any{"text": "stale edit", "expectedRevision": 1}, 409)
	ownerEvents := decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events?limit=1", 200))
	if len(ownerEvents.Events) != 1 || !ownerEvents.HasMore || ownerEvents.NextEventSeq != 1 {
		t.Fatalf("replay first page: %+v", ownerEvents)
	}
	ownerEvents = decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events?afterEventSeq=1", 200))
	if len(ownerEvents.Events) != 1 || ownerEvents.Events[0].EventSeq != 4 || ownerEvents.NextEventSeq != 4 {
		t.Fatalf("private events leaked or cursor skipped public event: %+v", ownerEvents)
	}
	memberEvents := decode[response.EventListResponse](t, f.get(t, &f.member, path+"/events", 200))
	if len(memberEvents.Events) != 4 {
		t.Fatalf("own events missing: %+v", memberEvents)
	}
	for _, privateText := range []string{"Hello member", "edited", "requestHash", "recipientUserId", "attempts"} {
		encoded, _ := json.Marshal(memberEvents)
		if strings.Contains(string(encoded), privateText) {
			t.Fatalf("replay leaked %s", privateText)
		}
	}
	f.send(t, &f.owner, "DELETE", messagePath, nil, 204)
	f.send(t, &f.owner, "DELETE", messagePath, nil, 204)
	originalAttempt := map[string]any{"clientMessageId": body["clientMessageId"], "text": "Hello member"}
	replayedDeleted := decode[response.MessageResponse](t, f.send(t, &f.owner, "POST", path+"/messages", originalAttempt, 200))
	if replayedDeleted.ID != first.ID || replayedDeleted.DeletedAt == nil {
		t.Fatal("replaying a deleted message resurrected/duplicated it")
	}
	f.send(t, &f.owner, "PATCH", messagePath, map[string]any{"text": "resurrect", "expectedRevision": 3}, 409)
	history := decode[response.MessageListResponse](t, f.get(t, &f.member, path+"/messages", 200))
	if len(history.Messages) != 1 || history.Messages[0].DeletedAt == nil || history.Messages[0].Text != nil || history.Messages[0].Revision != 3 {
		t.Fatalf("tombstone: %+v", history)
	}
	unmuted := decode[response.ConversationResponse](t, f.send(t, &f.member, "PATCH", path+"/preferences", map[string]any{"mutedUntil": nil}, 200))
	if unmuted.MutedUntil != nil || unmuted.LastReadSeq != 1 {
		t.Fatal("unmute lost read state")
	}
	var persisted chatModel.Conversation
	if err := f.db.First(&persisted, "id = ?", c.ID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.NextMessageSeq != 2 || persisted.NextEventSeq != 7 {
		t.Fatalf("unexpected sequence allocation: %+v", persisted)
	}
	if err := f.db.Delete(&f.member).Error; err != nil {
		t.Fatal(err)
	}
	visibleAfterDeletion := decode[response.EventListResponse](t, f.get(t, &f.owner, path+"/events", 200))
	if len(visibleAfterDeletion.Events) != 3 {
		t.Fatal("recipient deletion made private events public")
	}
}

func TestChatWritesRollbackWhenOutboxInsertFails(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	path := "/conversations/" + c.ID.String()
	first := decode[response.MessageResponse](t, f.send(t, &f.owner, "POST", path+"/messages", messageBody("original"), 201))
	const callback = "test:fail_chat_outbox"
	injected := errors.New("outbox unavailable")
	if err := f.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "outbox_events" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.db.Callback().Create().Remove(callback) })
	for _, operation := range []struct {
		method, suffix string
		body           any
	}{
		{"POST", "/messages", messageBody("must roll back")},
		{"PATCH", "/messages/" + first.ID.String(), map[string]any{"text": "must roll back", "expectedRevision": 1}},
		{"DELETE", "/messages/" + first.ID.String(), nil},
		{"PUT", "/read", map[string]any{"lastReadSeq": 1}},
		{"PATCH", "/preferences", map[string]any{"mutedUntil": time.Now().UTC().Add(time.Hour)}},
	} {
		f.send(t, &f.owner, operation.method, path+operation.suffix, operation.body, 503)
	}
	var message chatModel.Message
	if err := f.db.First(&message, "id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if message.Text == nil || *message.Text != "original" || message.Revision != 1 || message.DeletedAt != nil {
		t.Fatalf("partial message mutation: %+v", message)
	}
	for _, check := range []struct {
		record any
		want   int64
	}{{&chatModel.Message{}, 1}, {&outbox.Event{}, 1}, {&chatModel.ConversationUserState{}, 0}} {
		var count int64
		if err := f.db.Model(check.record).Count(&count).Error; err != nil || count != check.want {
			t.Fatalf("partial transaction %T: count=%d err=%v", check.record, count, err)
		}
	}
	var conversation chatModel.Conversation
	if err := f.db.First(&conversation, "id = ?", c.ID).Error; err != nil {
		t.Fatal(err)
	}
	if conversation.NextMessageSeq != 2 || conversation.NextEventSeq != 2 {
		t.Fatal("failed mutations consumed sequences")
	}
}

func TestConcurrentRetriesAndEditsHaveOneWinner(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	path := "/conversations/" + c.ID.String() + "/messages"
	body, _ := json.Marshal(messageBody("concurrent create"))
	token, err := authService.CreateToken(&f.owner)
	if err != nil {
		t.Fatal(err)
	}
	run := func(method, suffix string, bodies [][]byte) []*httptest.ResponseRecorder {
		results := make([]*httptest.ResponseRecorder, len(bodies))
		var workers sync.WaitGroup
		for i, b := range bodies {
			workers.Add(1)
			go func(i int, b []byte) {
				defer workers.Done()
				req := httptest.NewRequest(method, "/v1"+path+suffix, bytes.NewReader(b))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				results[i] = httptest.NewRecorder()
				f.router.ServeHTTP(results[i], req)
			}(i, b)
		}
		workers.Wait()
		return results
	}
	var id uuid.UUID
	created := 0
	for _, w := range run("POST", "", [][]byte{body, body, body, body, body, body, body, body}) {
		if w.Code != 200 && w.Code != 201 {
			t.Fatalf("retry failed: %d %s", w.Code, w.Body.String())
		}
		if w.Code == 201 {
			created++
		}
		msg := decode[response.MessageResponse](t, w)
		if id == uuid.Nil {
			id = msg.ID
		}
		if msg.ID != id || msg.Seq != 1 {
			t.Fatal("concurrent retries created different messages")
		}
	}
	if created != 1 {
		t.Fatalf("created=%d, want 1", created)
	}
	winners := 0
	for _, w := range run("PATCH", "/"+id.String(), [][]byte{[]byte(`{"text":"first edit","expectedRevision":1}`), []byte(`{"text":"second edit","expectedRevision":1}`)}) {
		if w.Code == 200 {
			winners++
		} else if w.Code != 409 {
			t.Fatalf("edit failed: %d %s", w.Code, w.Body.String())
		}
	}
	if winners != 1 {
		t.Fatal("stale revision edits did not serialize")
	}
	var events int64
	if err := f.db.Model(&outbox.Event{}).Count(&events).Error; err != nil || events != 2 {
		t.Fatalf("duplicate/missing outbox: count=%d err=%v", events, err)
	}
}

func TestMessageValidationAndCrossConversationPermissions(t *testing.T) {
	f := chatDatabase(t)
	first, second := f.trip(t), f.trip(t)
	path := "/conversations/" + first.ID.String()
	otherPath := "/conversations/" + second.ID.String()
	msg := decode[response.MessageResponse](t, f.send(t, &f.owner, "POST", otherPath+"/messages", messageBody("other conversation"), 201))
	badReply := messageBody("reply")
	badReply["replyToMessageId"] = msg.ID
	f.send(t, &f.owner, "POST", path+"/messages", badReply, 404)
	f.send(t, &f.owner, "PATCH", path+"/messages/"+msg.ID.String(), map[string]any{"text": "cross chat", "expectedRevision": 1}, 404)
	f.send(t, &f.owner, "DELETE", path+"/messages/"+msg.ID.String(), nil, 404)
	for _, invalid := range []any{map[string]any{}, messageBody(" \n\t"), messageBody(strings.Repeat("界", 4001)), map[string]any{"clientMessageId": uuid.Nil, "text": "zero UUID"}, map[string]any{"clientMessageId": uuid.New(), "text": "ignored file", "attachments": []string{"unsupported"}}} {
		f.send(t, &f.owner, "POST", path+"/messages", invalid, 400)
	}
	f.send(t, &f.owner, "POST", path+"/messages", messageBody(strings.Repeat("x", 33000)), 413)
	f.send(t, nil, "POST", path+"/messages", messageBody("unauthenticated"), 401)
	f.send(t, &f.outsider, "POST", path+"/messages", messageBody("outside"), 404)
	f.send(t, &f.owner, "PUT", path+"/read", map[string]any{}, 400)
	f.send(t, &f.owner, "PUT", path+"/read", map[string]any{"lastReadSeq": 1}, 400)
	f.send(t, &f.owner, "PATCH", path+"/preferences", map[string]any{}, 400)
	f.send(t, &f.owner, "PATCH", path+"/preferences", map[string]any{"mutedUntil": time.Now().Add(-time.Hour)}, 400)
	f.get(t, &f.owner, path+"/events?afterEventSeq=1", 400)
	f.get(t, &f.owner, path+"/events?afterEventSeq=-1", 400)
	f.get(t, &f.owner, path+"/events?limit=201", 400)
	memberMsg := decode[response.MessageResponse](t, f.send(t, &f.member, "POST", path+"/messages", messageBody("member message"), 201))
	f.send(t, &f.owner, "PATCH", path+"/messages/"+memberMsg.ID.String(), map[string]any{"text": "leader cannot rewrite member", "expectedRevision": 1}, 403)
	f.send(t, &f.owner, "DELETE", path+"/messages/"+memberMsg.ID.String(), nil, 204)
	if err := f.db.Model(&tripModel.TripMember{}).Where("trip_id = ? AND user_id = ?", first.TripID, f.member.ID).Update("status", enum.MembershipKicked).Error; err != nil {
		t.Fatal(err)
	}
	f.send(t, &f.member, "POST", path+"/messages", messageBody("kicked"), 404)
	f.send(t, &f.member, "PUT", path+"/read", map[string]any{"lastReadSeq": 1}, 404)
	f.send(t, &f.member, "PATCH", path+"/preferences", map[string]any{"mutedUntil": nil}, 404)
	f.get(t, &f.member, path+"/events", 404)
}
