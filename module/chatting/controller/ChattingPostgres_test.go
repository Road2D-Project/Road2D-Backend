package controller

import (
	"context"
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
	conversationModel "Road-To-Destination-BE/module/conversation/model"
	conversationRepository "Road-To-Destination-BE/module/conversation/repository"
	conversationService "Road-To-Destination-BE/module/conversation/service"
	"Road-To-Destination-BE/module/share/configuration"
	tripModel "Road-To-Destination-BE/module/trip/model"
	tripRepository "Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/utils/enum"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func TestConversationSocketAdmissionUsesCurrentPostgresRoster(t *testing.T) {
	db := testutil.Postgres(t)
	t.Setenv("SECRET_JWT", "chatting-access-test-secret-01234567890123456789")
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	owner := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test", SessionVersion: 1}
	outsider := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test", SessionVersion: 1}
	for _, user := range []*authModel.User{&owner, &outsider} {
		if err := db.Create(user).Error; err != nil {
			t.Fatal(err)
		}
	}
	trip := &tripModel.Trip{OwnerID: owner.ID, Name: "Public trip, private chat", Visibility: true,
		Status: enum.TripPlanning, MemberLimit: 5, InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "chatting-test"}
	joined := time.Now().UTC()
	if err := tripRepository.NewTripRepository(db).CreateTripWithMembers(t.Context(), trip,
		[]tripModel.TripMember{{UserID: owner.ID, Role: enum.TripRoleLeader, Status: enum.MembershipActive, JoinedAt: &joined}}, nil); err != nil {
		t.Fatal(err)
	}
	var conversation conversationModel.Conversation
	if err := db.First(&conversation, "trip_id = ?", trip.ID).Error; err != nil {
		t.Fatal(err)
	}
	rooms, err := chatting.NewChattingHubs(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := rooms.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	ctrl, err := NewChattingController(middleware.NewAuthenticationMiddleware(authRepository.NewUserRepository(db)),
		conversationService.NewConversationService(conversationRepository.NewConversationRepository(db)), rooms, nil)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	ctrl.RegisterRoutes(router.Group("/v1"))
	srv := httptest.NewServer(router)
	defer srv.Close()
	path := "/v1/realtime/conversations/" + conversation.ID.String()
	token, err := authService.CreateToken(&owner)
	if err != nil {
		t.Fatal(err)
	}
	header := http.Header{"Authorization": []string{"Bearer " + token}}
	for _, status := range []enum.TripStatus{enum.TripPlanning, enum.TripLocked, enum.TripActive, enum.TripStay, enum.TripCompleted} {
		if err := db.Model(trip).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		conn, rejected, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, header)
		if err != nil {
			if rejected != nil {
				_ = rejected.Body.Close()
			}
			t.Fatalf("active member blocked during %s: %v", status, err)
		}
		if hint := readSocketEvent(t, conn); hint.Type != "chat.syncRequired" {
			t.Fatal("missing join hint")
		}
		_ = conn.Close()
	}
	deny := func(rawToken string) {
		t.Helper()
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("Authorization", "Bearer "+rawToken)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("inactive admission = %d %s", recorder.Code, recorder.Body.String())
		}
	}
	outsiderToken, err := authService.CreateToken(&outsider)
	if err != nil {
		t.Fatal(err)
	}
	deny(outsiderToken)
	for _, status := range []enum.MembershipStatus{enum.MembershipInvited, enum.MembershipPending, enum.MembershipRejected, enum.MembershipLeft, enum.MembershipKicked} {
		if err := db.Model(&tripModel.TripMember{}).Where("trip_id = ? AND user_id = ?", trip.ID, owner.ID).Update("status", status).Error; err != nil {
			t.Fatal(err)
		}
		deny(token)
	}
}
