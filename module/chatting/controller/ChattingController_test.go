package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"Road-To-Destination-BE/middleware"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authService "Road-To-Destination-BE/module/authentication/service"
	"Road-To-Destination-BE/module/chatting"
	"Road-To-Destination-BE/module/conversation/model/response"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	"Road-To-Destination-BE/module/realtime"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestPersonalRoomUsesAuthenticatedUserAndAcceptsPreparedSummaries(t *testing.T) {
	f := newSocketFixture(t, nil)
	conn := f.dial(t, "/me?userId="+f.bob.ID.String(), f.token(t, &f.alice), true)
	defer conn.Close()
	if hint := readSocketEvent(t, conn); hint.Type != "chat.syncRequired" || strings.Contains(string(hint.Payload), "conversationId") {
		t.Fatalf("personal hint = %#v", hint)
	}
	_ = readSocketEvent(t, conn) // Own presence.
	ctx := context.Background()
	event := realtime.ServerEvent{Type: "chat.conversationUpdated", Payload: json.RawMessage(`{"unreadCount":3,"summaryVersion":8}`)}
	if accepted, err := f.rooms.Personal.PublishToRoom(ctx, f.bob.ID.String(), event); accepted || err != nil {
		t.Fatalf("client selected another user's room: %v, %v", accepted, err)
	}
	if accepted, err := f.rooms.Personal.PublishToRoom(ctx, f.alice.ID.String(), event); !accepted || err != nil {
		t.Fatalf("own room publication = %v, %v", accepted, err)
	}
	if got := readSocketEvent(t, conn).Type; got != "chat.conversationUpdated" {
		t.Fatalf("summary type = %s", got)
	}
	select {
	case <-f.reader.calls:
		t.Fatal("personal join queried conversation membership")
	default:
	}
}

func TestConversationSocketsExchangeTypingAndScopedPreparedEvents(t *testing.T) {
	f := newSocketFixture(t, nil)
	path := "/conversations/" + f.conversationID.String()
	first := f.dial(t, path, f.token(t, &f.alice), false)
	defer first.Close()
	_ = readSocketEvent(t, first) // Sync hint.
	_ = readSocketEvent(t, first) // Own presence.
	second := f.dial(t, path, f.token(t, &f.bob), false)
	defer second.Close()
	_ = readSocketEvent(t, second)
	_ = readSocketEvent(t, second)
	_ = readSocketEvent(t, first) // Bob's presence.
	for _, actor := range []uuid.UUID{f.alice.ID, f.bob.ID} {
		call := <-f.reader.calls
		if call.actor != actor || call.conversation != f.conversationID {
			t.Fatalf("wrong admission identity: %#v", call)
		}
	}
	if err := first.WriteJSON(realtime.Envelope{Type: "chat.typing", Payload: json.RawMessage(`{"typing":true}`)}); err != nil {
		t.Fatal(err)
	}
	for _, conn := range []*websocket.Conn{first, second} {
		frame := readSocketEvent(t, conn)
		var typing struct {
			UserID uuid.UUID `json:"userId"`
		}
		if frame.Type != "chat.typing" || json.Unmarshal(frame.Payload, &typing) != nil || typing.UserID != f.alice.ID {
			t.Fatalf("typing actor = %#v", frame)
		}
	}
	ctx := context.Background()
	for _, event := range []realtime.ServerEvent{
		{Type: "chat.readUpdated", RecipientUserID: f.alice.ID},
		{Type: "chat.messageCreated", Payload: json.RawMessage(`{"eventId":"source","eventSeq":4}`)},
	} {
		if accepted, err := f.rooms.Conversations.PublishToRoom(ctx, f.conversationID.String(), event); !accepted || err != nil {
			t.Fatalf("publish = %v, %v", accepted, err)
		}
	}
	if got := readSocketEvent(t, first).Type; got != "chat.readUpdated" {
		t.Fatalf("private event = %s", got)
	}
	if got := readSocketEvent(t, first).Type; got != "chat.messageCreated" {
		t.Fatalf("public event = %s", got)
	}
	if got := readSocketEvent(t, second).Type; got != "chat.messageCreated" {
		t.Fatalf("private event leaked to second member: %s", got)
	}
}

func TestChattingRejectsRequestsBeforeWebSocketUpgrade(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		path     string
		err      error
		tokenUse string
		origin   string
		want     int
	}{
		{name: "missing auth", path: "/me", tokenUse: "none", want: 401},
		{name: "refresh token", path: "/me", tokenUse: "refresh", want: 401},
		{name: "expired", path: "/me", tokenUse: "expired", want: 401},
		{name: "revoked session version", path: "/me", tokenUse: "revoked", want: 401},
		{name: "bad origin", path: "/me", origin: "https://untrusted.test", want: 403},
		{name: "bad UUID", path: "/conversations/not-a-uuid", want: 400},
		{name: "private resource", path: "/conversations/", err: conversationRepository.ErrConversationNotFound, want: 404},
		{name: "database failure", path: "/conversations/", err: errors.New("private database details"), want: 503},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			f := newSocketFixture(t, scenario.err)
			path := scenario.path
			if path == "/conversations/" {
				path += f.conversationID.String()
			}
			token := f.token(t, &f.alice)
			switch scenario.tokenUse {
			case "none":
				token = ""
			case "refresh":
				var err error
				token, err = authService.CreateRefreshToken(&f.alice)
				if err != nil {
					t.Fatal(err)
				}
			case "expired":
				token = f.signedToken(t, time.Now().Add(-time.Minute), 1)
			case "revoked":
				token = f.signedToken(t, time.Now().Add(time.Hour), 2)
			}
			req := httptest.NewRequest(http.MethodGet, "/v1/realtime"+path, nil)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			req.Header.Set("Origin", "https://app.example.test")
			if scenario.origin != "" {
				req.Header.Set("Origin", scenario.origin)
			}
			recorder := httptest.NewRecorder()
			f.router.ServeHTTP(recorder, req)
			if recorder.Code != scenario.want || strings.Contains(recorder.Body.String(), "private database details") {
				t.Fatalf("HTTP = %d %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestChattingClosesEstablishedSocketAtJWTExpiry(t *testing.T) {
	f := newSocketFixture(t, nil)
	expires := time.Now().Truncate(time.Second).Add(2 * time.Second)
	conn := f.dial(t, "/me", f.signedToken(t, expires, 1), false)
	defer conn.Close()
	_ = readSocketEvent(t, conn)
	_ = readSocketEvent(t, conn)
	_ = conn.SetReadDeadline(expires.Add(time.Second))
	_, _, err := conn.ReadMessage()
	var timeout net.Error
	if err == nil || (errors.As(err, &timeout) && timeout.Timeout()) {
		t.Fatalf("server failed to close at token expiry: %v", err)
	}
}

type accessCall struct{ actor, conversation uuid.UUID }
type stubConversationReader struct {
	err   error
	calls chan accessCall
}

func (r *stubConversationReader) GetConversation(_ context.Context, actor, id uuid.UUID) (*response.ConversationResponse, error) {
	r.calls <- accessCall{actor, id}
	if r.err != nil {
		return nil, r.err
	}
	return &response.ConversationResponse{ID: id}, nil
}

type socketUserRepository struct{ users map[string]*authModel.User }

func (r socketUserRepository) FindUserByUsername(_ context.Context, username string) (*authModel.User, error) {
	return r.users[username], nil
}

func (socketUserRepository) IsLoginSessionRevoked(context.Context, string) (bool, error) {
	return false, nil
}

type socketFixture struct {
	alice, bob     authModel.User
	conversationID uuid.UUID
	reader         *stubConversationReader
	rooms          *chatting.ChattingHubs
	router         *gin.Engine
	server         *httptest.Server
}

func newSocketFixture(t *testing.T, accessErr error) *socketFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	t.Setenv("SECRET_JWT", "chatting-access-test-secret-01234567890123456789")
	t.Setenv("SECRET_REFRESH_JWT", "chatting-refresh-test-secret-01234567890123456789")
	f := &socketFixture{conversationID: uuid.New(), reader: &stubConversationReader{err: accessErr, calls: make(chan accessCall, 8)}}
	f.alice = authModel.User{Username: "alice", SessionVersion: 1}
	f.alice.ID = uuid.New()
	f.bob = authModel.User{Username: "bob", SessionVersion: 1}
	f.bob.ID = uuid.New()
	var err error
	f.rooms, err = chatting.NewChattingHubs(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := f.rooms.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	auth := middleware.NewAuthenticationMiddleware(socketUserRepository{users: map[string]*authModel.User{"alice": &f.alice, "bob": &f.bob}})
	ctrl, err := NewChattingController(auth, f.reader, f.rooms, []string{"https://app.example.test"})
	if err != nil {
		t.Fatal(err)
	}
	f.router = gin.New()
	f.router.Use(middleware.LiftAccessToken())
	ctrl.RegisterRoutes(f.router.Group("/v1"))
	f.server = httptest.NewServer(f.router)
	t.Cleanup(f.server.Close)
	return f
}

func (f *socketFixture) token(t *testing.T, user *authModel.User) string {
	t.Helper()
	token, err := authService.CreateToken(user)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (f *socketFixture) signedToken(t *testing.T, expiry time.Time, version int64) string {
	t.Helper()
	claims := jwt.MapClaims{"sub": f.alice.ID.String(), "username": f.alice.Username, "sessionVersion": version,
		"tokenUse": "access", "jti": uuid.NewString(), "exp": expiry.Unix()}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("chatting-access-test-secret-01234567890123456789"))
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func (f *socketFixture) dial(t *testing.T, path, token string, queryAuth bool) *websocket.Conn {
	t.Helper()
	headers := http.Header{"Origin": []string{"https://app.example.test"}}
	if queryAuth {
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		path += separator + "access_token=" + token
	} else {
		headers.Set("Authorization", "Bearer "+token)
	}
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/v1/realtime"+path, headers)
	if err != nil {
		if response != nil {
			_ = response.Body.Close()
		}
		t.Fatalf("socket handshake failed: %v", err)
	}
	return conn
}

func readSocketEvent(t *testing.T, conn *websocket.Conn) realtime.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var event realtime.Envelope
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	return event
}
