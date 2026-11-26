package controller_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Road-To-Destination-BE/internal/testutil"
	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	authService "Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/conversation/controller"
	conversationModel "Road-To-Destination-BE/module/conversation/model"
	"Road-To-Destination-BE/module/conversation/model/response"
	"Road-To-Destination-BE/module/share/configuration"
	tripModel "Road-To-Destination-BE/module/trip/model"
	tripRepo "Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils"
	"Road-To-Destination-BE/utils/enum"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type chatFixture struct {
	db                      *gorm.DB
	router                  *gin.Engine
	owner, member, outsider authModel.User
	joined                  time.Time
}

func chatDatabase(t *testing.T) *chatFixture {
	t.Helper()
	t.Setenv("SECRET_JWT", "chat-access-test-secret-01234567890123456789")
	f := &chatFixture{db: testutil.Postgres(t), joined: time.Now().UTC().Truncate(time.Microsecond)}
	if err := configuration.AutoMigrate(f.db); err != nil {
		t.Fatal(err)
	}
	for _, user := range []*authModel.User{&f.owner, &f.member, &f.outsider} {
		*user = authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test", SessionVersion: 1}
		if err := f.db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	f.router = gin.New()
	auth := middleware.NewAuthenticationMiddleware(authRepo.NewUserRepository(f.db))
	controller.NewChatController(f.db, auth, nil).RegisterRoutes(f.router.Group("/v1"))
	return f
}

func (f *chatFixture) trip(t *testing.T) conversationModel.Conversation {
	t.Helper()
	trip := &tripModel.Trip{OwnerID: f.owner.ID, Name: "Private chat in discoverable trip", Visibility: true,
		Status: enum.TripPlanning, MemberLimit: 15, InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	members := []tripModel.TripMember{
		{UserID: f.owner.ID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: &f.joined},
		{UserID: f.member.ID, Role: enum.TripRoleMember, Status: enum.MembershipActive, JoinedAt: &f.joined},
	}
	if err := tripRepo.NewTripRepository(f.db).CreateTripWithMembers(t.Context(), trip, members, nil); err != nil {
		t.Fatal(err)
	}
	var conversation conversationModel.Conversation
	if err := f.db.First(&conversation, "trip_id = ?", trip.ID).Error; err != nil {
		t.Fatal(err)
	}
	return conversation
}

func (f *chatFixture) get(t *testing.T, user *authModel.User, path string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/v1"+path, nil)
	if user != nil {
		token, err := authService.CreateToken(user)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.router.ServeHTTP(w, req)
	if w.Code != status {
		t.Fatalf("GET %s: status=%d want=%d body=%s", path, w.Code, status, w.Body.String())
	}
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestChatReadsEnforceMembershipAndPreserveReadState(t *testing.T) {
	f := chatDatabase(t)
	c := f.trip(t)
	for seq := int64(1); seq <= 6; seq++ {
		text, attempt := "private message", uuid.New()
		msg := conversationModel.Message{Base: utils.Base{CreatedAt: f.joined.Add(time.Duration(seq) * time.Minute)},
			ConversationID: c.ID, SenderID: &f.member.ID, Seq: seq, Kind: enum.MessageKindText,
			Text: &text, ClientMessageID: &attempt, RequestHash: strings.Repeat("a", 64)}
		switch seq {
		case 1:
			msg.CreatedAt = f.joined.Add(-time.Minute)
		case 3:
			msg.SenderID = &f.owner.ID
		case 4:
			msg.Kind = enum.MessageKindSystem
		case 5:
			msg.DeletedAt, msg.Text = &f.joined, nil
		}
		if err := f.db.Create(&msg).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := f.db.Model(&c).Updates(map[string]any{"next_message_seq": 7, "next_event_seq": 10}).Error; err != nil {
		t.Fatal(err)
	}
	path := "/conversations/" + c.ID.String()
	w := f.get(t, &f.owner, path, 200)
	summary := decode[response.ConversationResponse](t, w)
	if summary.UnreadCount != 2 || summary.LastReadSeq != 1 || summary.LastMessageSeq != 6 || summary.SummaryVersion != 9 || summary.LastMessagePreview != nil || summary.ActiveMemberCount != 2 {
		t.Fatalf("incorrect summary: %+v", summary)
	}
	if w.Header().Get("Cache-Control") != "private, no-store" || strings.Contains(w.Body.String(), "private message") {
		t.Fatal("unread preview/caching policy violated")
	}
	history := decode[response.MessageListResponse](t, f.get(t, &f.owner, path+"/messages?limit=2", 200))
	if len(history.Messages) != 2 || history.Messages[0].Seq != 5 || history.Messages[1].Seq != 6 || !history.HasMore || history.NextBeforeSeq == nil || *history.NextBeforeSeq != 5 || history.Messages[0].Text != nil {
		t.Fatalf("latest page: %+v", history)
	}
	older := decode[response.MessageListResponse](t, f.get(t, &f.owner, path+"/messages?beforeSeq=5&limit=2", 200))
	if len(older.Messages) != 2 || older.Messages[0].Seq != 3 || older.Messages[1].Seq != 4 {
		t.Fatalf("older page: %+v", older)
	}
	w = f.get(t, &f.owner, path+"/messages?afterSeq=0&limit=2", 200)
	catchup := decode[response.MessageListResponse](t, w)
	if len(catchup.Messages) != 2 || catchup.Messages[0].Seq != 1 || catchup.Messages[1].Seq != 2 || catchup.NextAfterSeq == nil || *catchup.NextAfterSeq != 2 {
		t.Fatalf("catch-up: %+v", catchup)
	}
	for _, internal := range []string{"requestHash", "clientMessageId", "password", "email"} {
		if strings.Contains(w.Body.String(), internal) {
			t.Fatalf("history leaked %s", internal)
		}
	}
	empty := decode[response.MessageListResponse](t, f.get(t, &f.owner, path+"/messages?afterSeq=6", 200))
	if empty.Messages == nil || len(empty.Messages) != 0 || empty.HasMore {
		t.Fatalf("empty history: %+v", empty)
	}
	var states int64
	if err := f.db.Model(&conversationModel.ConversationUserState{}).Count(&states).Error; err != nil || states != 0 {
		t.Fatalf("GET mutated read state: count=%d err=%v", states, err)
	}
	state := conversationModel.ConversationUserState{ConversationID: c.ID, UserID: f.owner.ID, LastReadSeq: 6}
	if err := f.db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	summary = decode[response.ConversationResponse](t, f.get(t, &f.owner, path, 200))
	if summary.UnreadCount != 0 || summary.LastMessagePreview == nil || summary.LastMessagePreview.Text == nil {
		t.Fatalf("read preview: %+v", summary)
	}
	f.get(t, nil, path, 401)
	f.get(t, &f.outsider, path, 404)
	f.get(t, &f.outsider, path+"/messages", 404)
	for _, status := range []enum.MembershipStatus{enum.MembershipInvited, enum.MembershipPending, enum.MembershipLeft, enum.MembershipKicked, enum.MembershipRejected} {
		if err := f.db.Model(&tripModel.TripMember{}).Where("trip_id = ? AND user_id = ?", c.TripID, f.owner.ID).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		f.get(t, &f.owner, path, 404)
		f.get(t, &f.owner, path+"/messages", 404)
		list := decode[response.ConversationListResponse](t, f.get(t, &f.owner, "/conversations", 200))
		if len(list.Conversations) != 0 {
			t.Fatal("inactive membership leaked a conversation")
		}
	}
}

func TestChatListUsesStableCursorAndRejectsInvalidPagination(t *testing.T) {
	f := chatDatabase(t)
	for i := 0; i < 3; i++ {
		c := f.trip(t)
		if err := f.db.Model(&c).Update("created_at", f.joined).Error; err != nil {
			t.Fatal(err)
		}
	}
	seen, cursor := map[uuid.UUID]bool{}, ""
	for i := 0; i < 3; i++ {
		page := decode[response.ConversationListResponse](t, f.get(t, &f.owner, "/conversations?limit=1&cursor="+cursor, 200))
		if len(page.Conversations) != 1 || seen[page.Conversations[0].ID] {
			t.Fatalf("duplicate/missing page: %+v", page)
		}
		seen[page.Conversations[0].ID], cursor = true, page.NextCursor
	}
	if cursor != "" {
		t.Fatal("last page has an extra cursor")
	}
	id := uuid.NewString()
	for _, query := range []string{"limit=-1", "limit=101", "limit=no", "beforeSeq=0", "afterSeq=-1", "beforeSeq=2&afterSeq=0", "afterSeq=9223372036854775808"} {
		f.get(t, &f.owner, fmt.Sprintf("/conversations/%s/messages?%s", id, query), 400)
	}
	for _, query := range []string{"limit=51", "limit=-1", "cursor=bad", "cursor=e30"} {
		f.get(t, &f.owner, "/conversations?"+query, 400)
	}
	f.get(t, &f.owner, "/conversations/not-a-uuid", 400)
	f.get(t, &f.owner, "/conversations/"+id, 404)
	f.get(t, &f.owner, "/conversations/"+id+"/messages", 404)
	// Authentication still uses the real database, but unavailable chat storage
	// must fail closed without leaking SQL or internal connection errors.
	auth := middleware.NewAuthenticationMiddleware(authRepo.NewUserRepository(f.db))
	controller.NewChatController(nil, auth, nil).RegisterRoutes(f.router.Group("/v1/unavailable"))
	w := f.get(t, &f.owner, "/unavailable/conversations", 503)
	if !strings.Contains(w.Body.String(), "Chat temporarily unavailable") {
		t.Fatalf("unexpected storage error response: %s", w.Body.String())
	}
}
