package version

import (
	"fmt"

	"gorm.io/gorm"
)

// V8_AddTripIdempotencyKey backfills a key on trips that already exist, then
// marks the columns NOT NULL. A new database has no trips table yet, so this
// step returns and AutoMigrate creates the columns from the model.
// Each old row gets its own id as the key and a hash no client request produces,
// so a later wizard key cannot replay a trip that was created before the column.
func V8_AddTripIdempotencyKey(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable("trips") {
		return nil
	}
	statements := []string{
		`ALTER TABLE trips ADD COLUMN IF NOT EXISTS idempotency_key varchar(64)`,
		`ALTER TABLE trips ADD COLUMN IF NOT EXISTS idempotency_hash varchar(64)`,
		`UPDATE trips SET idempotency_key = id::text WHERE idempotency_key IS NULL OR idempotency_key = ''`,
		`UPDATE trips SET idempotency_hash = 'legacy' WHERE idempotency_hash IS NULL OR idempotency_hash = ''`,
		`ALTER TABLE trips ALTER COLUMN idempotency_key SET NOT NULL`,
		`ALTER TABLE trips ALTER COLUMN idempotency_hash SET NOT NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_trip_owner_idempotency ON trips (owner_id, idempotency_key)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return fmt.Errorf("trip idempotency key: %w", err)
		}
	}
	return nil
}
