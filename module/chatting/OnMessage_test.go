package chatting

import (
	"encoding/json"
	"testing"
	"testing/synctest"
	"time"

	"Road-To-Destination-BE/module/chatting/model/message"
	"Road-To-Destination-BE/module/realtime"
	"github.com/google/uuid"
)

func TestConversationSyncHintsNeverInventReplayWatermarks(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		actor := uuid.New()
		module := &ChattingCoordinator{id: uuid.New(), members: make(map[uuid.UUID]memberState)}
		s := &recordSession{}
		module.OnJoin(s, actor)
		hint, ok := s.frames[0].payload.(message.SyncRequired)
		if !ok || hint.Scope != "conversation" || hint.ConversationID == nil || *hint.ConversationID != module.id || s.frames[0].target != actor {
			t.Fatalf("join hint = %#v", s.frames)
		}
		module.OnMessage(s, actor, "resync", nil)
		if len(s.frames) != 1 {
			t.Fatal("resync flood was not throttled")
		}
		time.Sleep(time.Second)
		module.OnMessage(s, actor, "resync", json.RawMessage(`{}`))
		if len(s.frames) != 2 || s.frames[1].kind != "chat.syncRequired" {
			t.Fatal("resync failed to send a hint")
		}
		module.OnMessage(s, actor, "messageCreated", json.RawMessage(`{"text":"no WS writes"}`))
		if s.frames[len(s.frames)-1].kind != realtime.MsgTypeError {
			t.Fatal("durable write command was accepted")
		}
	})
}

func TestTypingUsesAuthenticatedIdentityExpiresAndStopsOnLeave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		actor := uuid.New()
		module := &ChattingCoordinator{id: uuid.New(), members: make(map[uuid.UUID]memberState)}
		s := &recordSession{}
		module.OnJoin(s, actor)
		for _, payload := range []string{`{}`, `null`, `{"typing":null}`, `{"typing":"yes"}`, `{"typing":true,"userId":"spoof"}`, `{"typing":true} {}`} {
			module.OnMessage(s, actor, "typing", json.RawMessage(payload))
			last := s.frames[len(s.frames)-1]
			if last.kind != realtime.MsgTypeError || last.target != actor {
				t.Fatalf("bad typing payload accepted: %s", payload)
			}
		}
		before := len(s.frames)
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":true}`))
		typing := s.frames[before].payload.(message.TypingEvent)
		if typing.UserID != actor || typing.ConversationID != module.id || !typing.Typing || !typing.ExpiresAt.Equal(time.Now().Add(typingLifetime)) || s.frames[before].target != uuid.Nil {
			t.Fatalf("typing = %#v", typing)
		}
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":true}`))
		if len(s.frames) != before+1 {
			t.Fatal("typing flood was not throttled")
		}
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":false}`))
		if s.frames[len(s.frames)-1].payload.(message.TypingEvent).Typing {
			t.Fatal("stop typing was throttled")
		}
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":true}`))
		if len(s.frames) != before+2 {
			t.Fatal("true/false toggles bypassed the rate bound")
		}
		time.Sleep(time.Second)
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":true}`))
		module.OnLeave(s, actor)
		if len(module.members) != 0 || s.frames[len(s.frames)-1].payload.(message.TypingEvent).Typing {
			t.Fatal("leave retained typing state")
		}
		afterLeave := len(s.frames)
		module.OnMessage(s, actor, "typing", json.RawMessage(`{"typing":true}`))
		if len(s.frames) != afterLeave {
			t.Fatal("offline sender could produce typing")
		}
	})
}

func TestPersonalChannelOnlyAcceptsResync(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		actor := uuid.New()
		module := &PersonalCoordinator{lastResync: make(map[uuid.UUID]time.Time)}
		s := &recordSession{}
		module.OnJoin(s, actor)
		hint := s.frames[0].payload.(message.SyncRequired)
		if hint.Scope != "personal" || hint.ConversationID != nil {
			t.Fatalf("personal hint = %#v", hint)
		}
		module.OnMessage(s, actor, "typing", nil)
		if s.frames[1].kind != realtime.MsgTypeError || s.frames[1].target != actor {
			t.Fatal("personal channel accepted typing")
		}
		time.Sleep(time.Second)
		module.OnMessage(s, actor, "resync", json.RawMessage(`{"conversationId":"spoof"}`))
		if s.frames[len(s.frames)-1].kind != realtime.MsgTypeError {
			t.Fatal("resync accepted an injected target")
		}
		module.OnMessage(s, actor, "resync", nil)
		if s.frames[len(s.frames)-1].payload.(message.SyncRequired).Scope != "personal" {
			t.Fatal("personal resync did not send a hint")
		}
		module.OnLeave(s, actor)
		if len(module.lastResync) != 0 {
			t.Fatal("personal module retained a disconnected user")
		}
	})
}

type recordedFrame struct {
	target  uuid.UUID
	kind    string
	payload any
}

type recordSession struct{ frames []recordedFrame }

func (s *recordSession) Send(actor uuid.UUID, kind string, payload any) {
	s.frames = append(s.frames, recordedFrame{actor, kind, payload})
}
func (s *recordSession) Broadcast(kind string, payload any) { s.Send(uuid.Nil, kind, payload) }
func (*recordSession) BroadcastLatest(string, string, any)  { panic("chat must not batch typing") }
func (*recordSession) IsOnline(uuid.UUID) bool              { return true }
