package model

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestAvatarKeyIsPersistedButNeverSerialized(t *testing.T) {
	parsed, err := schema.Parse(&User{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	field := parsed.FieldsByDBName["avatar_url"]
	if field == nil || field.Name != "AvatarPath" || !field.Creatable || !field.Updatable || !field.Readable {
		t.Fatal("avatar column does not map to permanent key")
	}
	key := "owner/private-key.png"
	viewURL := "https://storage.example/view"
	data, err := json.Marshal(&User{AvatarPath: &key, AvatarURL: &viewURL})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), key) || !strings.Contains(string(data), "avatarUrl") {
		t.Fatalf("unexpected JSON %s", data)
	}
	urlField := parsed.FieldsByName["AvatarURL"]
	if urlField.Creatable || urlField.Updatable || urlField.Readable {
		t.Fatal("viewing URL would be persisted")
	}
}
