package version

import "gorm.io/gorm"

// Existing users start at version 1. Pre-version tokens require a fresh login.
func V10_AddUserSessionVersion(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("users") {
		return nil
	}
	return db.Exec(`ALTER TABLE users ADD COLUMN IF NOT EXISTS session_version bigint NOT NULL DEFAULT 1`).Error
}
