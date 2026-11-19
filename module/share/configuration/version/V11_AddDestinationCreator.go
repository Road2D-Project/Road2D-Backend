package version

import "gorm.io/gorm"

// Legacy attached pins retain trip-based permissions. Unattributed standalone
// pins stay private until their ownership can be established outside this API.
func V11_AddDestinationCreator(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("destinations") {
		return nil
	}
	return db.Exec(`ALTER TABLE destinations ADD COLUMN IF NOT EXISTS created_by_user_id uuid`).Error
}
