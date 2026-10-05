package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Road-To-Destination-BE/module/authentication/model"
	"Road-To-Destination-BE/module/realtime"
	"Road-To-Destination-BE/module/share"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func TestJoinRoomRejectsNonMemberBeforeUpgrade(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewLobbyController(nil, realtime.NewHub(), stubAuthorizer{err: realtime.ErrNotMember})
	user := &model.User{}
	user.ID = uuid.New()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime/rooms/trip", nil)
	c.Params = gin.Params{{Key: "roomId", Value: "not-a-member"}}
	c.Set("currentUser", user)

	ctrl.HandleJoinRoom()(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var body share.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "not a member of this room" {
		t.Fatalf("message %q", body.Message)
	}
}

func TestJoinRoomRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctrl := NewLobbyController(nil, realtime.NewHub(), stubAuthorizer{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/realtime/rooms/trip", nil)
	c.Params = gin.Params{{Key: "roomId", Value: uuid.New().String()}}

	ctrl.HandleJoinRoom()(c)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", w.Code)
	}
}

func TestLiftAccessTokenOnlyWhenHeaderMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/?access_token=abc", nil)

	liftAccessToken()(c)
	if got := c.GetHeader("Authorization"); got != "Bearer abc" {
		t.Fatalf("authorization %q", got)
	}

	c.Request.Header.Set("Authorization", "Bearer header")
	liftAccessToken()(c)
	if got := c.GetHeader("Authorization"); got != "Bearer header" {
		t.Fatalf("header overwritten: %q", got)
	}
}

type stubAuthorizer struct {
	err error
}

func (s stubAuthorizer) AuthorizeRoom(context.Context, uuid.UUID, string) error {
	return s.err
}
