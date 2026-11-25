package version

import "gorm.io/gorm"

// V12_AddChatFoundation runs after model DDL, on fresh and existing databases.
// Keep backfill in the same transaction as the additional reply/content guards.
func V12_AddChatFoundation(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_message_conversation_id
			ON messages (conversation_id, id)`).Error; err != nil {
			return err
		}
		constraints := []struct{ name, definition string }{
			{"fk_message_reply_conversation", `FOREIGN KEY (conversation_id, reply_to_message_id)
				REFERENCES messages (conversation_id, id) ON UPDATE CASCADE ON DELETE NO ACTION`},
			{"chk_message_text", `CHECK (deleted_at IS NOT NULL OR kind <> 'text' OR
				(text IS NOT NULL AND char_length(text) BETWEEN 1 AND 4000 AND text ~ '[^[:space:]]'))`},
			{"chk_message_tombstone", `CHECK (deleted_at IS NULL OR text IS NULL)`},
			{"chk_message_client_attempt", `CHECK (kind = 'system' OR
				(client_message_id IS NOT NULL AND char_length(request_hash) = 64))`},
		}
		for _, constraint := range constraints {
			if !tx.Migrator().HasConstraint("messages", constraint.name) {
				if err := tx.Exec("ALTER TABLE messages ADD CONSTRAINT " + constraint.name + " " + constraint.definition).Error; err != nil {
					return err
				}
			}
		}
		return tx.Exec(`INSERT INTO conversations
			(id, trip_id, next_message_seq, next_event_seq, created_at, updated_at)
			SELECT gen_random_uuid(), id, 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP FROM trips
			ON CONFLICT (trip_id) DO NOTHING`).Error
	})
}
