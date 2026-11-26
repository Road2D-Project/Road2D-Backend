package repository_test

import (
	"errors"
	"strings"
	"testing"

	"Road-To-Destination-BE/internal/testutil"
	authModel "Road-To-Destination-BE/module/authentication/model"
	authRepo "Road-To-Destination-BE/module/authentication/repository"
	conversationModel "Road-To-Destination-BE/module/conversation/model"
	friendModel "Road-To-Destination-BE/module/friend/model"
	friendRepo "Road-To-Destination-BE/module/friend/repository"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/module/trip/model/request"
	"Road-To-Destination-BE/module/trip/repository"
	"Road-To-Destination-BE/module/trip/service"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func chatTripDatabase(t *testing.T) (*gorm.DB, authModel.User) {
	t.Helper()
	db := testutil.Postgres(t)
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	owner := chatTripUser(t, db)
	return db, owner
}

func chatTripUser(t *testing.T, db *gorm.DB) authModel.User {
	t.Helper()
	user := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

func TestTripCreateReplayAndForkHaveIndependentConversations(t *testing.T) {
	db, owner := chatTripDatabase(t)
	member := chatTripUser(t, db)
	low, high := friendModel.OrderedPair(owner.ID, member.ID)
	friend := friendModel.Friendship{UserLowID: low, UserHighID: high, RequesterID: owner.ID, Status: enum.FriendStatusAccepted}
	if err := db.Create(&friend).Error; err != nil {
		t.Fatal(err)
	}
	trips, members := repository.NewTripRepository(db), repository.NewTripMemberRepository(db)
	svc := service.NewTripService(trips, authRepo.NewUserRepository(db), members, members,
		nil, nil, friendRepo.NewFriendshipRepository(db))
	tier := enum.TripTypeBronze
	req := request.CreateTripRequest{Name: "Trip chat", IdempotencyKey: uuid.NewString(), TripType: &tier, MemberUserIDs: []uuid.UUID{member.ID}}
	created, inserted, err := svc.CreateTrip(t.Context(), &owner, req)
	if err != nil || !inserted {
		t.Fatalf("create trip: inserted=%v err=%v", inserted, err)
	}
	replayed, inserted, err := svc.CreateTrip(t.Context(), &owner, req)
	if err != nil || inserted || replayed.ID != created.ID {
		t.Fatalf("replay trip: inserted=%v err=%v", inserted, err)
	}
	var source conversationModel.Conversation
	if err := db.First(&source, "trip_id = ?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	text, clientID := "Source history", uuid.New()
	message := conversationModel.Message{ConversationID: source.ID, SenderID: &owner.ID, Seq: 1,
		Kind: enum.MessageKindText, Text: &text, ClientMessageID: &clientID, RequestHash: strings.Repeat("a", 64)}
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	forked, err := svc.ForkTrip(t.Context(), &owner, created.ID, request.ForkTripRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var target conversationModel.Conversation
	if err := db.First(&target, "trip_id = ?", forked.ID).Error; err != nil {
		t.Fatal(err)
	}
	if source.ID == target.ID {
		t.Fatal("fork reused the source conversation")
	}
	var count int64
	if err := db.Model(&conversationModel.Conversation{}).Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("create/replay/fork conversations: count=%d err=%v", count, err)
	}
	if err := db.Model(&conversationModel.Message{}).Where("conversation_id = ?", target.ID).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("fork must not copy messages: count=%d err=%v", count, err)
	}
}

func TestTripChatFailureRollsBackTripRosterAndInitialGraph(t *testing.T) {
	db, owner := chatTripDatabase(t)
	injected := errors.New("conversation write failed")
	const callback = "test:reject_conversation"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "conversations" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Callback().Create().Remove(callback) })
	trip := &model.Trip{OwnerID: owner.ID, Name: "Rollback", Status: enum.TripPlanning,
		MemberLimit: 15, InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	roster := []model.TripMember{{UserID: owner.ID, Role: enum.TripRoleLeader, Status: enum.MembershipActive}}
	main := &model.InitialMainBranch{Destinations: []model.Destination{
		{Name: "Start", Lat: 10, Lng: 106, Status: enum.Editing},
		{Name: "Finish", Lat: 11, Lng: 107, Status: enum.Editing},
	}}
	err := repository.NewTripRepository(db).CreateTripWithMembers(t.Context(), trip, roster, main)
	if !errors.Is(err, injected) {
		t.Fatalf("got %v, want injected conversation failure", err)
	}
	for _, record := range []any{&model.Trip{}, &model.TripMember{}, &model.TripBranch{}, &model.BranchDestination{}, &model.Destination{}, &conversationModel.Conversation{}} {
		var count int64
		if err := db.Model(record).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("partial commit for %T: count=%d err=%v", record, count, err)
		}
	}
}
