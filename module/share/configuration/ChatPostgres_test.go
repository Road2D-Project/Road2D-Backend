package configuration_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"Road-To-Destination-BE/internal/testutil"
	authModel "Road-To-Destination-BE/module/authentication/model"
	chatModel "Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/share/configuration"
	"Road-To-Destination-BE/module/share/outbox"
	tripModel "Road-To-Destination-BE/module/trip/model"
	"Road-To-Destination-BE/utils/enum"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func legacyChatTrip(t *testing.T, db *gorm.DB) (uuid.UUID, uuid.UUID) {
	t.Helper()
	user := authModel.User{Username: uuid.NewString(), Email: uuid.NewString() + "@example.test", Password: "test"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	trip := tripModel.Trip{OwnerID: user.ID, Name: "Chat test", Status: enum.TripPlanning,
		InviteToken: uuid.NewString(), IdempotencyKey: uuid.NewString(), IdempotencyHash: "test"}
	if err := db.Create(&trip).Error; err != nil {
		t.Fatal(err)
	}
	return trip.ID, user.ID
}

func requireSQLState(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != code {
		t.Fatalf("got %v, want PostgreSQL SQLSTATE %s", err, code)
	}
}

func TestChatMigrationBackfillsExistingTripsWithoutResettingCounters(t *testing.T) {
	db := testutil.Postgres(t)
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	tripID, _ := legacyChatTrip(t, db)
	if err := db.Migrator().DropTable(&outbox.Event{}, &chatModel.Message{}, &chatModel.ConversationUserState{}, &chatModel.Conversation{}); err != nil {
		t.Fatal(err)
	}
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	var conversation chatModel.Conversation
	if err := db.First(&conversation, "trip_id = ?", tripID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&conversation).Update("next_message_seq", 7).Error; err != nil {
		t.Fatal(err)
	}
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	var persisted chatModel.Conversation
	if err := db.First(&persisted, "trip_id = ?", tripID).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ID != conversation.ID || persisted.NextMessageSeq != 7 || persisted.NextEventSeq != 1 {
		t.Fatalf("backfill rewrote existing conversation: %+v", persisted)
	}
}

func TestChatSchemaScopesRepliesAttemptsAndDeletion(t *testing.T) {
	db := testutil.Postgres(t)
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	tripID, ownerID := legacyChatTrip(t, db)
	otherTripID, _ := legacyChatTrip(t, db)
	if err := configuration.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	var first, other chatModel.Conversation
	if err := db.First(&first, "trip_id = ?", tripID).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.First(&other, "trip_id = ?", otherTripID).Error; err != nil {
		t.Fatal(err)
	}
	requireSQLState(t, db.Create(chatModel.NewConversation(tripID)).Error, "23505")
	requireSQLState(t, db.Create(chatModel.NewConversation(uuid.New())).Error, "23503")
	newMessage := func(conversationID uuid.UUID, seq int64) chatModel.Message {
		clientID, text := uuid.New(), "Hello"
		return chatModel.Message{ConversationID: conversationID, SenderID: &ownerID, Seq: seq,
			Kind: enum.MessageKindText, Text: &text, ClientMessageID: &clientID, RequestHash: strings.Repeat("a", 64)}
	}
	message := newMessage(first.ID, 1)
	if err := db.Create(&message).Error; err != nil {
		t.Fatal(err)
	}
	reply := newMessage(first.ID, 2)
	reply.ReplyToMessageID = &message.ID
	if err := db.Create(&reply).Error; err != nil {
		t.Fatalf("second message with same-conversation reply: %v", err)
	}
	duplicate := newMessage(first.ID, 3)
	duplicate.ClientMessageID = message.ClientMessageID
	requireSQLState(t, db.Create(&duplicate).Error, "23505")
	crossReply := newMessage(other.ID, 1)
	crossReply.ReplyToMessageID = &message.ID
	requireSQLState(t, db.Create(&crossReply).Error, "23503")
	blank, whitespace := newMessage(first.ID, 3), " \n\t"
	blank.Text = &whitespace
	requireSQLState(t, db.Create(&blank).Error, "23514")
	state := chatModel.ConversationUserState{ConversationID: first.ID, UserID: ownerID}
	if err := db.Create(&state).Error; err != nil {
		t.Fatal(err)
	}
	duplicateState := chatModel.ConversationUserState{ConversationID: first.ID, UserID: ownerID}
	requireSQLState(t, db.Create(&duplicateState).Error, "23505")
	requireSQLState(t, db.Model(&state).Update("last_read_seq", -1).Error, "23514")
	seq := int64(1)
	event := outbox.Event{Type: "chat.messageCreated", AggregateID: message.ID,
		ConversationID: &first.ID, EventSeq: &seq, Payload: json.RawMessage(`{}`)}
	cleanup := outbox.Event{Type: "media.delete", AggregateID: message.ID, Payload: json.RawMessage(`{}`)}
	for _, pending := range []*outbox.Event{&event, &cleanup} {
		if err := db.Create(pending).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Delete(&tripModel.Trip{}, "id = ?", tripID).Error; err != nil {
		t.Fatalf("delete trip including messages that reply to one another: %v", err)
	}
	for _, record := range []any{&first, &message, &reply, &state, &event} {
		if err := db.First(record).Error; !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("cascade left a record (%T): %v", record, err)
		}
	}
	if err := db.First(&cleanup).Error; err != nil {
		t.Fatalf("independent cleanup task must survive: %v", err)
	}
}
