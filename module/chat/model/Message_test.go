package model_test

import (
	"sync"
	"testing"

	"Road-To-Destination-BE/module/chat/model"
	"Road-To-Destination-BE/module/share/outbox"

	"gorm.io/gorm/schema"
)

// A unique conversation-only index would accidentally permit just one message.
func TestMessageUniquenessIsScopedToSequenceOrClientAttempt(t *testing.T) {
	parsed, err := schema.Parse(&model.Message{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string][]string{}
	for _, index := range parsed.ParseIndexes() {
		if index.Class != "UNIQUE" {
			continue
		}
		for _, field := range index.Fields {
			keys[index.Name] = append(keys[index.Name], field.DBName)
		}
		if len(index.Fields) == 1 && index.Fields[0].DBName == "conversation_id" {
			t.Fatal("a conversation must support multiple messages")
		}
	}
	for name, want := range map[string][]string{
		"idx_message_seq":    {"conversation_id", "seq"},
		"idx_message_client": {"conversation_id", "sender_id", "client_message_id"},
	} {
		got := keys[name]
		if len(got) != len(want) {
			t.Fatalf("%s: got %v, want %v", name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}
}

func TestChatRelationshipsParseWithoutLegacyGroupModels(t *testing.T) {
	cache := &sync.Map{}
	for _, record := range []any{&model.Conversation{}, &model.ConversationUserState{}, &model.Message{}, &outbox.Event{}} {
		parsed, err := schema.Parse(record, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatal(err)
		}
		for _, relation := range parsed.Relationships.Relations {
			if relation.FieldSchema.Table == "groups" || relation.FieldSchema.Table == "group_members" {
				t.Fatalf("%s depends on a legacy group relationship", parsed.Table)
			}
		}
	}
}
