package version

import "gorm.io/gorm"

func V13_AddOutboxLifecycle(db *gorm.DB) error {
	for _, sql := range []string{
		`CREATE INDEX IF NOT EXISTS idx_outbox_pending_conversation ON outbox_events (conversation_id, event_seq) WHERE dispatched_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_outbox_retention ON outbox_events (created_at, id) WHERE dispatched_at IS NOT NULL`,
	} {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}
