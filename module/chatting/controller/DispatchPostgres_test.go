package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Road-To-Destination-BE/internal/testutil"
	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRepository "Road-To-Destination-BE/module/authentication/repository"
	authService "Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/chatting"
	conversationController "Road-To-Destination-BE/module/conversation/controller"
	conversationModel "Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/conversation/model/response"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	conversationService "Road-To-Destination-BE/module/conversation/service"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/share/outbox"
	tripModel "Road-To-Destination-BE/module/trip/model"
	tripRepository "Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func TestCommittedMessagesReadStateAndEditsReachCorrectSocketChannels(t *testing.T) {
	f := dispatchDatabase(t)
	chat := f.connect(t, &f.member, false)
	defer chat.Close()
	personal := f.connect(t, &f.member, true)
	defer personal.Close()
	created := f.send(t, &f.owner, "POST", f.path()+"/messages", map[string]any{"clientMessageId": uuid.NewString(), "text": "private message content"}, 201)
	var message response.MessageResponse
	if err := json.Unmarshal(created, &message); err != nil {
		t.Fatal(err)
	}
	f.flush(t)
	frame := readSocketEvent(t, chat)
	if frame.Type != "chat.messageCreated" || bytes.Contains(frame.Payload, []byte("private message content")) {
		t.Fatalf("message invalidation = %s", frame.Payload)
	}
	badge := readBadge(t, personal)
	if badge.UnreadCount != 1 || badge.LastMessageSeq != 1 {
		t.Fatalf("badge=%+v", badge)
	}
	var source outbox.Event
	if err := f.db.Where("type = ?", conversationModel.EventMessageCreated).First(&source).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.delivery.Handle(t.Context(), source); err != nil {
		t.Fatal(err)
	} // Retry must dedup each channel separately.
	ownerChat := f.connect(t, &f.owner, false)
	defer ownerChat.Close()
	_ = readSocketEvent(t, chat) // Owner presence.
	f.send(t, &f.member, "PUT", f.path()+"/read", map[string]any{"lastReadSeq": 1}, 200)
	f.flush(t)
	if got := readSocketEvent(t, chat).Type; got != "chat.readUpdated" {
		t.Fatalf("private read event=%s", got)
	}
	if badge := readBadge(t, personal); badge.UnreadCount != 0 {
		t.Fatalf("read badge=%+v", badge)
	}
	f.send(t, &f.owner, "PATCH", f.path()+"/messages/"+message.ID.String(), map[string]any{"text": "edited private content", "expectedRevision": 1}, 200)
	f.flush(t)
	if got := readSocketEvent(t, ownerChat).Type; got != "chat.messageUpdated" {
		t.Fatalf("private read leaked to owner: %s", got)
	}
	if got := readSocketEvent(t, chat).Type; got != "chat.messageUpdated" {
		t.Fatalf("edit invalidation=%s", got)
	}
	if badge := readBadge(t, personal); badge.UnreadCount != 0 || badge.LastMessageRevision == nil || *badge.LastMessageRevision != 2 {
		t.Fatalf("edit badge=%+v", badge)
	}
	f.send(t, &f.owner, "DELETE", f.path()+"/messages/"+message.ID.String(), nil, 204)
	f.flush(t)
	if got := readSocketEvent(t, chat).Type; got != "chat.messageDeleted" {
		t.Fatalf("delete invalidation=%s", got)
	}
	if badge := readBadge(t, personal); badge.UnreadCount != 0 {
		t.Fatalf("delete badge=%+v", badge)
	}
}

func TestDelayedSourceEventUsesLatestAuthoritativeBadge(t *testing.T) {
	f := dispatchDatabase(t)
	personal := f.connect(t, &f.member, true)
	defer personal.Close()
	f.send(t, &f.owner, "POST", f.path()+"/messages", map[string]any{"clientMessageId": uuid.NewString(), "text": "already read"}, 201)
	f.send(t, &f.member, "PUT", f.path()+"/read", map[string]any{"lastReadSeq": 1}, 200)
	f.flush(t)
	for range 2 {
		badge := readBadge(t, personal)
		if badge.UnreadCount != 0 || badge.LastReadSeq != 1 || badge.SummaryVersion != 2 {
			t.Fatalf("late event undid read acknowledgement: %+v", badge)
		}
	}
}

func TestOfflineDispatchCompletesWithoutCreatingRooms(t *testing.T) {
	f := dispatchDatabase(t)
	f.send(t, &f.owner, "POST", f.path()+"/messages", map[string]any{"clientMessageId": uuid.NewString(), "text": "offline"}, 201)
	f.flush(t)
	for _, target := range []struct {
		hub *realtime.Hub
		id  string
	}{
		{f.hubs.Conversations, f.scope.ID.String()}, {f.hubs.Personal, f.owner.ID.String()}, {f.hubs.Personal, f.member.ID.String()},
	} {
		if exists, err := target.hub.HasRoom(t.Context(), target.id); exists || err != nil {
			t.Fatalf("offline dispatch created room: %v %v", exists, err)
		}
	}
}

func TestPartialHandoffRetriesOnlyMissingChannel(t *testing.T) {
	f := dispatchDatabase(t)
	// Delivery visits recipients in UUID order. Fail the personal channel only
	// after the first recipient's conversation handoff has been accepted.
	target := &f.member
	if f.owner.ID.String() < f.member.ID.String() {
		target = &f.owner
	}
	chat := f.connect(t, target, false)
	defer chat.Close()
	f.send(t, &f.owner, "POST", f.path()+"/messages", map[string]any{"clientMessageId": uuid.NewString(), "text": "partial"}, 201)
	var event outbox.Event
	if err := f.db.Where("type = ?", conversationModel.EventMessageCreated).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.hubs.Personal.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := f.delivery.Handle(t.Context(), event); !errors.Is(err, realtime.ErrRoomClosed) {
		t.Fatalf("partial failure=%v", err)
	}
	if got := readSocketEvent(t, chat).Type; got != "chat.messageCreated" {
		t.Fatalf("first channel=%s", got)
	}
	options := realtime.DefaultRoomOptions()
	options.TickInterval = 0
	personal, err := realtime.NewHubWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	if err := personal.Mount("chat", chatting.NewPersonalModule()); err != nil {
		t.Fatal(err)
	}
	if err := f.hubs.WithAccess(t.Context(), func() error { f.hubs.Personal = personal; return nil }); err != nil {
		t.Fatal(err)
	}
	me := f.connect(t, target, true)
	defer me.Close()
	if err := f.delivery.Handle(t.Context(), event); err != nil {
		t.Fatal(err)
	}
	expectedUnread := int64(1)
	if target.ID == f.owner.ID {
		expectedUnread = 0
	}
	if badge := readBadge(t, me); badge.UnreadCount != expectedUnread {
		t.Fatalf("missing personal handoff=%+v", badge)
	}
	if accepted, err := f.hubs.Conversations.PublishToRoom(t.Context(), f.scope.ID.String(), realtime.ServerEvent{Type: "chat.messageUpdated", RecipientUserID: target.ID}); !accepted || err != nil {
		t.Fatalf("marker=%v %v", accepted, err)
	}
	if got := readSocketEvent(t, chat).Type; got != "chat.messageUpdated" {
		t.Fatalf("retry duplicated accepted channel: %s", got)
	}
}

func TestKickEvictsBeforeReturningAndRemovesPersonalConversation(t *testing.T) {
	f := dispatchDatabase(t)
	chat := f.connect(t, &f.member, false)
	defer chat.Close()
	personal := f.connect(t, &f.member, true)
	defer personal.Close()
	members := tripRepository.NewTripMemberRepository(f.db).WithAccessLifecycle(f.lifecycle)
	member, err := members.FindMemberById(t.Context(), f.member.ID, f.scope.TripID)
	if err != nil {
		t.Fatal(err)
	}
	if err := members.ApplyKick(t.Context(), member); err != nil {
		t.Fatal(err)
	}
	assertSocketClosed(t, chat)
	f.flush(t)
	removed := readSocketEvent(t, personal)
	if removed.Type != "chat.conversationRemoved" || bytes.Contains(removed.Payload, []byte("unreadCount")) {
		t.Fatalf("removal=%s %s", removed.Type, removed.Payload)
	}
	f.send(t, &f.member, "GET", f.path(), nil, 404)
}

func TestTripDeletionKeepsIndependentRemovalEventAfterCascade(t *testing.T) {
	f := dispatchDatabase(t)
	chat := f.connect(t, &f.member, false)
	defer chat.Close()
	personal := f.connect(t, &f.member, true)
	defer personal.Close()
	if err := tripRepository.NewTripRepository(f.db).WithAccessLifecycle(f.lifecycle).DeleteTrip(t.Context(), f.scope.TripID); err != nil {
		t.Fatal(err)
	}
	assertSocketClosed(t, chat)
	var event outbox.Event
	if err := f.db.Where("type = ?", outbox.ConversationDeleted).First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.ConversationID != nil {
		t.Fatal("deletion event depended on deleted conversation")
	}
	f.flush(t)
	if got := readSocketEvent(t, personal).Type; got != "chat.conversationRemoved" {
		t.Fatalf("delete removal=%s", got)
	}
}

func TestLogoutRevokesOnlyItsLoginAndOldEventsCannotEvictReplacement(t *testing.T) {
	f := dispatchDatabase(t)
	chat := f.connect(t, &f.member, false)
	defer chat.Close()
	personal := f.connect(t, &f.member, true)
	defer personal.Close()
	old := f.credentials[f.member.ID]
	store := authRepository.NewRefreshRevokeStore(nil, f.db).WithAccessLifecycle(f.lifecycle)
	logout := authService.NewLogoutService(store, authRepository.NewUserRepository(f.db))
	if err := logout.Logout(t.Context(), old.refresh); err != nil {
		t.Fatal(err)
	}
	assertSocketClosed(t, chat)
	assertSocketClosed(t, personal)
	f.send(t, &f.member, "GET", f.path(), nil, 401)
	if err := logout.Logout(t.Context(), old.refresh); err != nil {
		t.Fatal(err)
	}
	f.credentials[f.member.ID] = loginPair(t, &f.member)
	fresh := f.connect(t, &f.member, true)
	defer fresh.Close()
	var revocation outbox.Event
	if err := f.db.Where("type = ?", outbox.SessionRevoked).First(&revocation).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.delivery.Handle(t.Context(), revocation); err != nil {
		t.Fatal(err)
	}
	accepted, err := f.hubs.Personal.PublishToRoom(t.Context(), f.member.ID.String(), realtime.ServerEvent{Type: "chat.conversationUpdated", Payload: json.RawMessage(`{"marker":"new login"}`)})
	if err != nil || !accepted {
		t.Fatalf("replacement publish=%v %v", accepted, err)
	}
	if got := readSocketEvent(t, fresh).Type; got != "chat.conversationUpdated" {
		t.Fatalf("replacement closed: %s", got)
	}
	claims, err := authService.VerifyRefreshToken(f.credentials[f.member.ID].refresh)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := logout.AssertRefreshUsable(t.Context(), f.credentials[f.member.ID].refresh); err != nil {
		t.Fatal(err)
	}
	if claims["sid"] == "" {
		t.Fatal("new login has no session ID")
	}
}

type credentialPair struct{ access, refresh string }

func TestFailedLifecycleOutboxWriteRollsBackAccessAndKeepsSocket(t *testing.T) {
	f := dispatchDatabase(t)
	conn := f.connect(t, &f.member, false)
	defer conn.Close()
	failed := errors.New("injected outbox failure")
	callback := "step9_fail_outbox"
	if err := f.db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "outbox_events" {
			tx.AddError(failed)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.db.Callback().Create().Remove(callback) })
	members := tripRepository.NewTripMemberRepository(f.db).WithAccessLifecycle(f.lifecycle)
	member, err := members.FindMemberById(t.Context(), f.member.ID, f.scope.TripID)
	if err != nil {
		t.Fatal(err)
	}
	if err := members.ApplyKick(t.Context(), member); !errors.Is(err, failed) {
		t.Fatalf("kick failure=%v", err)
	}
	if err := tripRepository.NewTripRepository(f.db).WithAccessLifecycle(f.lifecycle).DeleteTrip(t.Context(), f.scope.TripID); !errors.Is(err, failed) {
		t.Fatalf("delete failure=%v", err)
	}
	if err := authRepository.NewUserRepository(f.db).WithAccessLifecycle(f.lifecycle).ResetPassword(t.Context(), f.member.ID, "NewPassword1!", 1); !errors.Is(err, failed) {
		t.Fatalf("reset failure=%v", err)
	}
	var user authModel.User
	if err := f.db.First(&user, "id = ?", f.member.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.SessionVersion != 1 || user.Password != "test" {
		t.Fatal("password/version escaped rollback")
	}
	member, err = members.FindMemberById(t.Context(), f.member.ID, f.scope.TripID)
	if err != nil || !member.Status.IsActive() {
		t.Fatal("membership escaped rollback")
	}
	var scope conversationModel.Conversation
	if err := f.db.First(&scope, "id = ?", f.scope.ID).Error; err != nil {
		t.Fatal(err)
	}
	if scope.NextEventSeq != 1 {
		t.Fatal("event watermark escaped rollback")
	}
	if accepted, err := f.hubs.Conversations.PublishToRoom(t.Context(), f.scope.ID.String(), realtime.ServerEvent{Type: "chat.messageCreated", RecipientUserID: f.member.ID}); !accepted || err != nil {
		t.Fatalf("socket was revoked despite rollback: %v %v", accepted, err)
	}
	_ = readSocketEvent(t, conn)
}

func TestPasswordResetRevokesOldVersionsButDelayedEventKeepsNewLogin(t *testing.T) {
	f := dispatchDatabase(t)
	conn := f.connect(t, &f.member, true)
	defer conn.Close()
	if err := authRepository.NewUserRepository(f.db).WithAccessLifecycle(f.lifecycle).ResetPassword(t.Context(), f.member.ID, "NewPassword1!", 1); err != nil {
		t.Fatal(err)
	}
	assertSocketClosed(t, conn)
	f.send(t, &f.member, "GET", f.path(), nil, 401)
	if err := f.db.First(&f.member, "id = ?", f.member.ID).Error; err != nil {
		t.Fatal(err)
	}
	f.credentials[f.member.ID] = loginPair(t, &f.member)
	fresh := f.connect(t, &f.member, true)
	defer fresh.Close()
	f.flush(t)
	if accepted, err := f.hubs.Personal.PublishToRoom(t.Context(), f.member.ID.String(), realtime.ServerEvent{Type: "chat.conversationUpdated"}); !accepted || err != nil {
		t.Fatalf("fresh login=%v %v", accepted, err)
	}
	_ = readSocketEvent(t, fresh)
}

func TestLegacyLogoutAdvancesAccountVersionOnlyOnce(t *testing.T) {
	f := dispatchDatabase(t)
	claims := jwt.MapClaims{"sub": f.member.ID.String(), "username": f.member.Username, "tokenUse": "refresh", "sessionVersion": 1, "jti": uuid.NewString(), "exp": time.Now().Add(time.Hour).Unix()}
	legacy, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("dispatch-refresh-secret-01234567890123456789"))
	if err != nil {
		t.Fatal(err)
	}
	logout := authService.NewLogoutService(authRepository.NewRefreshRevokeStore(nil, f.db).WithAccessLifecycle(f.lifecycle), authRepository.NewUserRepository(f.db))
	for range 2 {
		if err := logout.Logout(t.Context(), legacy); err != nil {
			t.Fatal(err)
		}
	}
	var user authModel.User
	if err := f.db.First(&user, "id = ?", f.member.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.SessionVersion != 2 {
		t.Fatalf("legacy version=%d", user.SessionVersion)
	}
}

type dispatchFixture struct {
	db            *gorm.DB
	owner, member authModel.User
	scope         conversationModel.Conversation
	hubs          *chatting.ChattingHubs
	lifecycle     *chatting.AccessLifecycle
	delivery      *chatting.Delivery
	dispatcher    *outbox.Dispatcher
	credentials   map[uuid.UUID]credentialPair
	router        *gin.Engine
	server        *httptest.Server
}

func dispatchDatabase(t *testing.T) *dispatchFixture {
	t.Helper()
	t.Setenv("SECRET_JWT", "dispatch-access-secret-01234567890123456789")
	t.Setenv("SECRET_REFRESH_JWT", "dispatch-refresh-secret-01234567890123456789")
	f := &dispatchFixture{db: testutil.Postgres(t), credentials: make(map[uuid.UUID]credentialPair)}
	if err := configuration.AutoMigrate(f.db); err != nil {
		t.Fatal(err)
	}
	for _, user := range []*authModel.User{&f.owner, &f.member} {
		*user = authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test", SessionVersion: 1}
		if err := f.db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
		f.credentials[user.ID] = loginPair(t, user)
	}
	trip := &tripModel.Trip{OwnerID: f.owner.ID, Name: "Dispatch", Status: enum.TripPlanning, MemberLimit: 5, InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	now := time.Now()
	if err := tripRepository.NewTripRepository(f.db).CreateTripWithMembers(t.Context(), trip, []tripModel.TripMember{
		{UserID: f.owner.ID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: &now},
		{UserID: f.member.ID, Role: enum.TripRoleMember, Status: enum.MembershipActive, JoinedAt: &now},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if err := f.db.First(&f.scope, "trip_id = ?", trip.ID).Error; err != nil {
		t.Fatal(err)
	}
	var err error
	f.hubs, err = chatting.NewChattingHubs(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := f.hubs.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	records := conversationRepository.NewConversationRepository(f.db)
	reads := conversationService.NewConversationService(records)
	f.lifecycle = chatting.NewAccessLifecycle(f.hubs, records)
	f.delivery = chatting.NewDelivery(f.lifecycle, reads)
	f.dispatcher = outbox.NewDispatcher(outbox.NewRepository(f.db), f.delivery)
	auth := middleware.NewAuthenticationMiddleware(authRepository.NewUserRepository(f.db))
	ctrl, err := NewChattingController(auth, reads, f.hubs, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.router = gin.New()
	f.router.Use(middleware.LiftAccessToken())
	ctrl.RegisterRoutes(f.router.Group("/v1"))
	conversationController.NewChatController(f.db, auth, nil).RegisterRoutes(f.router.Group("/v1"))
	f.server = httptest.NewServer(f.router)
	t.Cleanup(f.server.Close)
	return f
}

func loginPair(t *testing.T, user *authModel.User) credentialPair {
	t.Helper()
	a, r, err := authService.CreateTokenPair(user)
	if err != nil {
		t.Fatal(err)
	}
	return credentialPair{a, r}
}
func (f *dispatchFixture) path() string { return "/conversations/" + f.scope.ID.String() }
func (f *dispatchFixture) connect(t *testing.T, user *authModel.User, personal bool) *websocket.Conn {
	t.Helper()
	path := "/conversations/" + f.scope.ID.String()
	if personal {
		path = "/me"
	}
	conn, rejected, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/v1/realtime"+path, http.Header{"Authorization": []string{"Bearer " + f.credentials[user.ID].access}})
	if err != nil {
		if rejected != nil {
			_ = rejected.Body.Close()
		}
		t.Fatalf("socket admission: %v", err)
	}
	if got := readSocketEvent(t, conn).Type; got != "chat.syncRequired" {
		t.Fatalf("join event=%s", got)
	}
	_ = readSocketEvent(t, conn)
	return conn
}
func (f *dispatchFixture) send(t *testing.T, user *authModel.User, method, path string, body any, want int) []byte {
	t.Helper()
	var encoded []byte
	if body != nil {
		var err error
		encoded, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, "/v1"+path, bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.credentials[user.ID].access)
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != want {
		t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
	}
	return w.Body.Bytes()
}
func (f *dispatchFixture) flush(t *testing.T) {
	t.Helper()
	for range 100 {
		n, err := f.dispatcher.ProcessBatch(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			var pending int64
			if err := f.db.Model(&outbox.Event{}).Where("dispatched_at IS NULL").Count(&pending).Error; err != nil {
				t.Fatal(err)
			}
			if pending != 0 {
				t.Fatalf("%d events failed handoff", pending)
			}
			return
		}
	}
	t.Fatal("dispatch did not settle")
}
func readBadge(t *testing.T, conn *websocket.Conn) struct {
	UnreadCount, LastReadSeq, LastMessageSeq, SummaryVersion int64
	LastMessageRevision                                      *int64
} {
	t.Helper()
	frame := readSocketEvent(t, conn)
	if frame.Type != "chat.conversationUpdated" || bytes.Contains(frame.Payload, []byte("private content")) || bytes.Contains(frame.Payload, []byte("lastMessagePreview")) {
		t.Fatalf("badge frame=%s %s", frame.Type, frame.Payload)
	}
	var badge struct {
		UnreadCount, LastReadSeq, LastMessageSeq, SummaryVersion int64
		LastMessageRevision                                      *int64
	}
	if err := json.Unmarshal(frame.Payload, &badge); err != nil {
		t.Fatal(err)
	}
	return badge
}
func assertSocketClosed(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err := conn.ReadMessage()
	var timeout net.Error
	if err == nil || errors.As(err, &timeout) && timeout.Timeout() {
		t.Fatalf("socket not closed: %v", err)
	}
}
